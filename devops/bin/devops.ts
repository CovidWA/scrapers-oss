#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from '@aws-cdk/core';
import {DevopsStack} from '../lib/devops-stack';

const AWS_REGION = 'us-west-2';
const env = {region: AWS_REGION};

const app = new cdk.App();
new DevopsStack(app, 'DevopsStack', {env});
