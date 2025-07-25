#!/usr/bin/env python3

import sys
import json
import os

if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--config":
        print("""
configVersion: v1
kubernetes:
- name: monitor-configmaps
  kind: ConfigMap
  labelSelector:
    matchLabels:
      smoke-test: "true"
  jqFilter: ".data.key"
  executeHookOnEvent: ["Added", "Modified"]
""")
    else:
        binding_context_path = os.environ.get("BINDING_CONTEXT_PATH")
        with open(binding_context_path) as f:
            binding_context = json.load(f)

        for bc in binding_context:
            if bc["type"] == "Event":
                cm_name = bc["object"]["metadata"]["name"]
                filtered_data = bc["filterResult"]
                print(f"ConfigMap '{cm_name}' data.key is '{filtered_data}'") 