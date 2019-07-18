The **controller** (controller.py) will be a server which clients will hit with requests for work. The controller will respond with descriptions of work to do.

1. controller creates a client
2. client makes GET request to /control
3. server responds with JSON blob that contains info about work to do
4. client does work (or may exit)
5. client makes POST request with JSON body to send test results


{'trace': bool, 'repeat': int, 'work': int, 'sleep': float (seconds), 'no_flush': bool, 'exit': bool}
