#!/usr/bin/python3

import json
import sys
import os
from urllib.request import urlopen, urlretrieve

component = "rdsys"
path = ""
if len(sys.argv) >= 2:
    path = sys.argv[1]
translations_url = f"https://hosted.weblate.org/api/components/tor/{component}/translations/"

with urlopen(translations_url) as response:
    translations = json.load(response)

for t in translations["results"]:
    if t["language_code"] == "en" or t["translated_percent"] < 90:
        continue
    urlretrieve(f'https://hosted.weblate.org/download/tor/{component}/{t["language_code"]}/', os.path.join(path, t["filename"]))
