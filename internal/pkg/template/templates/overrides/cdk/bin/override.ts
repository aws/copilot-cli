#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib';
import { TransformedStack } from '../stack';

const app = new cdk.App();
new TransformedStack(app, 'Stack');