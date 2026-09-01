---
name: Report Builder
description: Build a text report with a reusable Python formatting module and save it in the conversation workspace.
---

# Report Builder

Use `build_report.py` to create a text report from a title and a list of items.

The entry script imports `report_format.py`, so both files must remain in the same Skill directory.

## Parameters

Pass a JSON object containing:

- `title`: report title.
- `items`: array of text items.
- `filename`: optional output filename, default `report.txt`.

## Output

The script writes the report into the current conversation workspace and returns its filename and content.
