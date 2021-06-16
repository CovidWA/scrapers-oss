import {Browser} from 'puppeteer-core';

import * as mhealthCredentials from '../mhealth_credentials.json';
import {
  ApiRequestCredentials,
  CredentialConsumingFunction,
  CredentialManager,
} from '../src/helpers/albertsonsCredentialManager';
import {log} from '../src/logger';

export class TestAlbertsonsCredentialManager implements CredentialManager {
  private credentials: ApiRequestCredentials;

  constructor() {
    this.credentials = {
      csrfKey: mhealthCredentials.csrfKey,
      cookies: [
        {name: 'JSESSIONID', value: mhealthCredentials.JSESSIONID},
        {name: 'AWSALB', value: mhealthCredentials.AWSALB},
        {name: 'AWSALBCORS', value: mhealthCredentials.AWSALBCORS},
        {name: 'AWSALBTG', value: mhealthCredentials.AWSALBTG},
        {name: 'AWSALBTGCORS', value: mhealthCredentials.AWSALBTGCORS},
      ],
    };

    log('Using the following credentials:');
    log(`csrfKey: ${this.credentials.csrfKey}`);
    this.credentials.cookies.forEach(({name, value}) => {
      log(`${name}: ${value}`);
    });
  }

  async useCredentials<T>(
    _browser: Browser,
    credentialConsumingFn: CredentialConsumingFunction<T>
  ): Promise<T> {
    let result;
    try {
      result = await credentialConsumingFn(this.credentials);
    } catch (err) {
      result = err;
    }

    console.log(result);
    return result;
  }
}
