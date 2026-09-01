---
name: Third-Party Dependency Demo
description: Demonstrate a Python Skill that imports a third-party package.
---

# Third-Party Dependency Demo

Use `fetch_data.py` when testing how the runtime handles a Skill with an external Python dependency.

## Parameters

Pass a JSON object containing an optional `message` field.

## Dependency

This Skill declares `requests` in `requirements.txt`. The default Coze Skill
runtime does not install this file automatically, so the script is expected to
fail with `ModuleNotFoundError` until the dependency is added to the runtime.

## Output

When `requests` is available, the script returns the message and the installed
Requests version as JSON.
