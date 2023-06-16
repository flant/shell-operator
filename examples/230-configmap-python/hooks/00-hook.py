#!/usr/bin/env python3

import sys
import kubernetes import client, config

if __name__ == "__main__":
    if len(sys.argv)>1 and sys.argv[1] == "--config":
        print('{"configVersion":"v1", "onStartup": 10}')
    else:
        print("OnStartup Python powered hook")
        config.load_incluster_config()
        
        v1 = client.CoreV1Api()
        metadata = client.V1ObjectMeta(name="config-map-for-proj")

        config_map = {
            "memory": "256Gi",
            "coreCPU": "128",
            "ver": "1.0.1"
        }

        config_map_body = client.V1ConfigMap(data=config_map, metadata=metadata)
        resp = v1.create_namespaced_config_map(namespace="default", body=config_map_body)
        print(resp)

