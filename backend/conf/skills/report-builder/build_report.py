import json
import pathlib
import sys

from report_format import format_report


params = json.loads(sys.argv[1])
filename = params.get("filename", "report.txt")
if pathlib.Path(filename).name != filename:
    raise ValueError("filename must not contain a directory")

content = format_report(params["title"], params["items"])
path = pathlib.Path.cwd() / filename
path.write_text(content, encoding="utf-8")
print(json.dumps({"filename": filename, "content": content}, ensure_ascii=False))
