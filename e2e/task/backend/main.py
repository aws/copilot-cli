import time
import os
import datetime

status = os.getenv("STATUS", "NOT OVERRIDDEN")
time.sleep(20)

out = f'e2e environment variables: {status}'
for i in range(8):
	print(out, flush=True)
	time.sleep(3)
