#!/usr/bin/env python3
"""Run with pg_virtualenv python3 -m unittest tools.test_release_status -v."""
import contextlib
import io
import subprocess
import sys
import unittest
from unittest import mock

from tools import release_status


class SchemaReleaseTest(unittest.TestCase):
    def psql(self, *args):
        return subprocess.run(["psql", "-X", "-v", "ON_ERROR_STOP=1", *args],
                              check=True, capture_output=True, text=True)

    def release_check(self):
        original_run = subprocess.run

        def run(command, **kwargs):
            if command[:5] == ["sudo", "-n", "-u", "postgres", "psql"]:
                command = ["psql", "-X", *command[10:]]
            return original_run(command, **kwargs)

        output = io.StringIO()
        with mock.patch.object(release_status.subprocess, "run", side_effect=run), \
             mock.patch.object(sys, "argv", ["release_status.py", "--check"]), \
             contextlib.redirect_stdout(output):
            code = release_status.main()
        return code, output.getvalue()

    def test_missing_then_applied_migrations(self):
        migrations = sorted((release_status.ROOT / "server/db").glob("[0-9][0-9][0-9]_*.sql"))
        for migration in migrations:
            if migration.name.startswith(("025_", "026_")):
                continue
            self.psql("-f", str(migration))
        code, output = self.release_check()
        print("Before 025/026 (exit %d):\n%s" % (code, output), end="")
        self.assertEqual(code, 1)
        self.assertIn("025_journal_grants.sql: missing SELECT on public.journal_entries", output)
        self.assertIn("025_journal_grants.sql: missing SELECT on public.mcp_tokens", output)
        self.assertIn("026_crash_reports.sql: missing table public.crash_reports", output)
        for migration in migrations:
            if migration.name.startswith(("025_", "026_")):
                self.psql("-f", str(migration))
        code, output = self.release_check()
        print("After 025/026 (exit %d):\n%s" % (code, output), end="")
        self.assertEqual(code, 0, output)


if __name__ == "__main__":
    unittest.main()
