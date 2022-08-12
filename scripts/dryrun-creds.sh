#! /usr/bin/env python3
import subprocess
import sys

cmd = subprocess.run(['aws', 'configure', 'list-profiles'], stdout=subprocess.PIPE)
profiles = cmd.stdout.decode('utf-8').split('\n')

if len(sys.argv) < 2:
  sys.exit('no arg passed to script, must be one of: "e2e" or "regression"')

if sys.argv[1] == "e2e":
  requiredProfiles = ['e2etestenv', 'e2eprodenv']
elif sys.argv[1] == "regression":
  requiredProfiles = ['regression']
else:
  sys.exit('invalid arg passed to script, must be one of: "e2e" or "regression"')

for profile in requiredProfiles:
  if profile in profiles:
    continue
  print(f'Profile [{profile}] is required but does not exist. Copying your [default] access key id and secret key...')
  cmd = subprocess.run(['aws', 'configure', 'get', 'default.region'], stdout=subprocess.PIPE)
  region = cmd.stdout.decode('utf-8').strip()
  cmd = subprocess.run(['aws', 'configure', 'get', 'default.aws_access_key_id'], stdout=subprocess.PIPE)
  access_key = cmd.stdout.decode('utf-8').strip()
  cmd = subprocess.run(['aws', 'configure', 'get', 'default.aws_secret_access_key'], stdout=subprocess.PIPE)
  secret = cmd.stdout.decode('utf-8').strip()
  subprocess.run(['aws', 'configure', 'set', 'region', region, '--profile', profile])
  subprocess.run(['aws', 'configure', 'set', 'aws_access_key_id', access_key, '--profile', profile])
  subprocess.run(['aws', 'configure', 'set', 'aws_secret_access_key', secret, '--profile', profile])