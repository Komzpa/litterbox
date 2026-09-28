import struct
import zlib
from email.message import EmailMessage
from email.utils import formatdate
from uuid import uuid4


def inline_image():
    """A visible 16x16 red PNG, not a 1x1 tracking-pixel lookalike."""
    def chunk(kind, data):
        return struct.pack("!I", len(data)) + kind + data + struct.pack("!I", zlib.crc32(kind + data))

    rows = (b"\x00" + bytes((210, 55, 45)) * 16) * 16
    return (b"\x89PNG\r\n\x1a\n"
            + chunk(b"IHDR", struct.pack("!IIBBBBB", 16, 16, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(rows)) + chunk(b"IEND", b""))


def build_fixture_messages(run_id, sender, recipients, remote_image_url):
    messages = []
    for label in ("LB1-A", "LB1-B", "LB1-C", "LB1-P1", "LB1-P2", "LB1-P3"):
        message = EmailMessage()
        message["Subject"] = f"{label} {run_id}"
        message.set_content(f"Acceptance fixture {label} for run {run_id}.")
        message["From"] = sender
        message["To"] = ", ".join(recipients)
        message["Date"] = formatdate(localtime=False)
        message["Message-ID"] = f"<lb1-{uuid4().hex}@litterbox.invalid>"
        messages.append((label, message))

    rich = EmailMessage()
    rich["Subject"] = f"LB1-RICH {run_id}"
    rich["From"] = sender
    rich["To"] = ", ".join(recipients)
    rich["Date"] = formatdate(localtime=False)
    rich["Message-ID"] = f"<lb1-rich-{uuid4().hex}@litterbox.invalid>"
    rich.set_content(f"Rich HTML acceptance fixture for run {run_id}; view the HTML alternative for images.")
    cid = f"lb1-inline-{uuid4().hex}@litterbox.invalid"
    html = ("<html><body><h1>Litterbox rich-mail fixture</h1>"
            "<p>This message tests HTML rendering and inline image handling.</p>"
            f"<img alt=\"Inline fixture image\" src=\"cid:{cid}\">"
            f"<img alt=\"Remote fixture image\" src=\"{remote_image_url}\">"
            "</body></html>")
    rich.add_alternative(html, subtype="html")
    html_part = rich.get_payload()[-1]
    html_part.add_related(
        inline_image(),
        maintype="image", subtype="png", cid=f"<{cid}>", disposition="inline", filename="pixel.png",
    )
    messages.append(("LB1-RICH", rich))
    return messages
