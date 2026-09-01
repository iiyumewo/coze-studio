import json
import pathlib
import sys


params = json.loads(sys.argv[1])
path = pathlib.Path.cwd() / "notes.txt"
with path.open("a", encoding="utf-8") as file:
    file.write(params["note"] + "\n")

notes = path.read_text(encoding="utf-8").splitlines()
print(json.dumps({"notes": notes}, ensure_ascii=False))
