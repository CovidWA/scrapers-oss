import {ApiRequestCredentials} from './albertsonsCredentialManager';
import {secret} from '../../config.json';
import fetch from 'node-fetch';

export type CachedCredentials = {
  [clinicKey: string]: {credentials?: ApiRequestCredentials; useCount: number};
};

export const retrieveCachedCredentials = async (): Promise<CachedCredentials> => {
  const response = await fetch('https://api.covidwa.com/v1/get_stored', {
    method: 'POST',
    headers: {'content-type': 'application/json'},
    body: JSON.stringify({secret}),
  });

  const {data} = await response.json();

  return data;
};

export const sendCredentialsToCache = async (
  credentials: CachedCredentials
): Promise<boolean> => {
  const response = await fetch('https://api.covidwa.com/v1/store', {
    method: 'POST',
    headers: {'content-type': 'application/json'},
    body: JSON.stringify({secret, data: credentials}),
  });

  return response.status === 200;
};
