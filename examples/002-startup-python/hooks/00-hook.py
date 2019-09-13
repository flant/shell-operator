#!/usr/bin/env python

import sys

if __name__ == "__main__":
    if len(sys.argv)>1 and sys.argv[1] == "--config":
        print '{"configVersion":"v1", "onStartup": 10}'
    else:
        print "OnStartup Python powered hook"
