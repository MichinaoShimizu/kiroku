"""Smoke tests for the scenario runner; no real model is invoked."""
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
RUNNER = ROOT / "scripts" / "decision_continuity_agent_eval.py"
REVISION = "a" * 40


class RunnerSmokeTests(unittest.TestCase):
    def invoke(self, agent, *extra):
        with tempfile.TemporaryDirectory() as directory:
            result = subprocess.run(
                [sys.executable, str(RUNNER), "--revision", REVISION,
                 "--agent-version", "fake-agent-test",
                 "--agent-argv-json", json.dumps(agent),
                 "--output", directory, "--timeout", "10", *extra],
                capture_output=True, text=True, timeout=40, check=False,
            )
            manifest_path = pathlib.Path(directory) / "manifest.json"
            manifest = json.loads(manifest_path.read_text()) if manifest_path.exists() else None
            responses = {p.name: p.read_text() for p in pathlib.Path(directory).glob("*.response.txt")}
            return result, manifest, responses

    def test_mock_agent_runs_selected_case_without_auto_pass(self):
        agent = [sys.executable, "-c", "import sys; print('Decision Log: rationale Unknown'); sys.stdin.read()"]
        result, manifest, responses = self.invoke(agent, "--case", "S01", "--skills-loaded")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(manifest["results"]), 1)
        self.assertEqual(manifest["results"][0]["id"], "S01")
        self.assertEqual(manifest["results"][0]["status"], "REVIEW_REQUIRED")
        self.assertIn("S01.response.txt", responses)

    def test_no_attestation_is_inconclusive(self):
        agent = [sys.executable, "-c", "print('Decision Log Unknown')"]
        result, manifest, _ = self.invoke(agent, "--case", "S01")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(manifest["results"][0]["status"], "INCONCLUSIVE")

    def test_failed_agent_is_inconclusive(self):
        agent = [sys.executable, "-c", "import sys; sys.exit(4)"]
        result, manifest, _ = self.invoke(agent, "--case", "S02", "--skills-loaded")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(manifest["results"][0]["exit_code"], 4)
        self.assertEqual(manifest["results"][0]["status"], "INCONCLUSIVE")

    def test_invalid_revision_rejected(self):
        agent = [sys.executable, "-c", "print('x')"]
        with tempfile.TemporaryDirectory() as directory:
            result = subprocess.run(
                [sys.executable, str(RUNNER), "--revision", "main",
                 "--agent-version", "fake", "--agent-argv-json", json.dumps(agent),
                 "--output", directory], capture_output=True, text=True, check=False,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("40-character SHA", result.stderr)


if __name__ == "__main__":
    unittest.main()
