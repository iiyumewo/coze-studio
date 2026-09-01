import json
import sys

import requests


params = json.loads(sys.argv[1]) if len(sys.argv) > 1 else {}
print(
    json.dumps(
        {
            "message": params.get("message", "third-party dependency is available"),
            "requests_version": requests.__version__,
        }
    )
)
