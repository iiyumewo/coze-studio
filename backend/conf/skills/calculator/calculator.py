import json
import sys


params = json.loads(sys.argv[1])
print(json.dumps({"sum": params["a"] + params["b"]}))
