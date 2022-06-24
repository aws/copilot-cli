#! /usr/bin/env python3
import subprocess

cmd = subprocess.run(['aws', 'configure', 'list-profiles'], stdout=subprocess.PIPE)
profiles = cmd.stdout.decode('utf-8').split('\n')

for e2eprofile in ['e2etestenv', 'e2eprodenv']:
  if e2eprofile in profiles:
    continue
  print(f'Profile [{e2eprofile}] is required but does not exist. Copying your [default] access key id and secret key...')
  cmd = subprocess.run(['aws', 'configure', 'get', 'default.region'], stdout=subprocess.PIPE)
  region = cmd.stdout.decode('utf-8').strip()
  cmd = subprocess.run(['aws', 'configure', 'get', 'default.aws_access_key_id'], stdout=subprocess.PIPE)
  access_key = cmd.stdout.decode('utf-8').strip()
  cmd = subprocess.run(['aws', 'configure', 'get', 'default.aws_secret_access_key'], stdout=subprocess.PIPE)
  secret = cmd.stdout.decode('utf-8').strip()
  subprocess.run(['aws', 'configure', 'set', 'region', region, '--profile', e2eprofile])
  subprocess.run(['aws', 'configure', 'set', 'aws_access_key_id', access_key, '--profile', e2eprofile])
  subprocess.run(['aws', 'configure', 'set', 'aws_secret_access_key', secret, '--profile', e2eprofile])