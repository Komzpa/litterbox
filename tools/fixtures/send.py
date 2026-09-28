#!/usr/bin/env python3
"""Build and send Litterbox R28 acceptance mail fixtures."""

import argparse
import json
import os
import re
import smtplib
import ssl
import sys
import tempfile
from email.utils import parseaddr
from pathlib import Path
from urllib.parse import urlsplit

from fixture_parts.messages import build_fixture_messages

PRIVATE_STATE = Path.home() / ".local" / "state" / "litterbox"
DEFAULT_MANIFEST = PRIVATE_STATE / "fixture-manifest.json"


def env_default(name, default=None):
    return os.environ.get(name, default)


def parse_args(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-id", required=True, help="run identifier appended to fixture subjects")
    parser.add_argument("--to", action="append", default=[], metavar="ADDRESS", help="recipient address; repeat for every connected account")
    parser.add_argument("--from-address", default=env_default("LITTERBOX_FIXTURE_FROM"), help="sender address (or LITTERBOX_FIXTURE_FROM)")
    parser.add_argument("--smtp-host", default=env_default("LITTERBOX_SMTP_HOST"), help="SMTP server (or LITTERBOX_SMTP_HOST)")
    parser.add_argument("--smtp-port", type=int, default=int(env_default("LITTERBOX_SMTP_PORT", "587")))
    parser.add_argument("--smtp-user", default=env_default("LITTERBOX_SMTP_USER"))
    parser.add_argument("--smtp-password", default=env_default("LITTERBOX_SMTP_PASSWORD"))
    parser.add_argument("--remote-image-url", default=env_default("LITTERBOX_FIXTURE_IMAGE_URL"), help="public HTTPS image URL for the rich mail (or LITTERBOX_FIXTURE_IMAGE_URL)")
    parser.add_argument("--output-dir", type=Path, default=PRIVATE_STATE / "fixture-eml", help="directory for --dry-run .eml files (outside repository by default)")
    parser.add_argument("--manifest", type=Path, default=DEFAULT_MANIFEST, help="private manifest path (default is outside the repository)")
    parser.add_argument("--dry-run", action="store_true", help="write .eml files without connecting to SMTP")
    args = parser.parse_args(argv)
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,63}", args.run_id):
        parser.error("--run-id must be 1-64 filename-safe ASCII letters, digits, dots, underscores, or hyphens")
    recipients = list(args.to)
    if not recipients and env_default("LITTERBOX_FIXTURE_TO"):
        recipients = [item.strip() for item in os.environ["LITTERBOX_FIXTURE_TO"].split(",") if item.strip()]
    if not recipients:
        parser.error("provide at least one --to recipient or LITTERBOX_FIXTURE_TO")
    args.recipients = recipients
    if not args.from_address:
        parser.error("provide --from-address or LITTERBOX_FIXTURE_FROM")
    if not args.dry_run and not args.smtp_host:
        parser.error("provide --smtp-host or LITTERBOX_SMTP_HOST unless using --dry-run")
    url = urlsplit(args.remote_image_url or "")
    if url.scheme != "https" or not url.hostname or url.username or url.password:
        parser.error("provide --remote-image-url or LITTERBOX_FIXTURE_IMAGE_URL with a public HTTPS image URL")
    if bool(args.smtp_user) != bool(args.smtp_password):
        parser.error("SMTP username and password must be provided together")
    for address in [args.from_address, *recipients]:
        if not parseaddr(address)[1] or "@" not in parseaddr(address)[1]:
            parser.error(f"invalid mail address: {address!r}")
    if not 1 <= args.smtp_port <= 65535:
        parser.error("--smtp-port must be between 1 and 65535")
    return args


def write_manifest(path, run_id, ids):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    try:
        manifest = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(manifest, dict) or not isinstance(manifest.get("runs", {}), dict):
            raise ValueError("manifest must contain a runs object")
    except FileNotFoundError:
        manifest = {"runs": {}}
    manifest.setdefault("runs", {})[run_id] = {"message_ids": ids}
    fd, temp_name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as stream:
            json.dump(manifest, stream, indent=2, sort_keys=True)
            stream.write("\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temp_name, path)
        os.chmod(path, 0o600)
    finally:
        if os.path.exists(temp_name):
            os.unlink(temp_name)


def main(argv=None):
    args = parse_args(argv)
    messages = build_fixture_messages(args.run_id, args.from_address, args.recipients, args.remote_image_url)
    message_ids = {}

    if args.dry_run:
        args.output_dir.mkdir(parents=True, exist_ok=True)
        for label, message in messages:
            path = args.output_dir / f"{label}-{args.run_id}.eml"
            path.write_bytes(message.as_bytes())
            message_ids[label] = message["Message-ID"]
            print(f"Wrote {path} {message['Message-ID']}")
    else:
        context = ssl.create_default_context()
        with smtplib.SMTP(args.smtp_host, args.smtp_port, timeout=30) as client:
            client.ehlo()
            client.starttls(context=context)
            client.ehlo()
            if args.smtp_user:
                client.login(args.smtp_user, args.smtp_password)
            for label, message in messages:
                client.send_message(message)
                message_ids[label] = message["Message-ID"]
                print(f"Sent {message['Subject']} {message['Message-ID']}")

    write_manifest(args.manifest, args.run_id, message_ids)
    print(f"Manifest: {args.manifest}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
