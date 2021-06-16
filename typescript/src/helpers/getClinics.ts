// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import {error} from '../logger';
import * as http from 'http';
import * as https from 'https';
import * as config from '../../config.json';
import fetch from 'node-fetch';

const secret = config.secret;
const getClinicsUrl = config.getClinicsServerUrl;
const getClinicsStatusUrl = config.getClinicsServerUrl.replace(
  /_internal$/,
  ''
);

export interface Clinic {
  id: string;
  name: string;
  Availability: string;
  city: string;
  county: string;
  address: string;
  url?: string;
  alternateUrl?: string;
  restrictions?: string;
  key: string;
  scraper_config: string;
}

interface ClinicCore {
  id: string;
  humanName: string;
  url: string;
  key: string;
  address: string;
  scraper_config: string;
}

interface ClinicStatus {
  id: string;
  Availability: string;
  lastChecked: number;
  lastAvailable: number;
}

export function getClinicsByUrlKeyword(
  keyword: string,
  clinics: Array<Clinic>
): Array<ClinicCore> {
  const coreClinicData: Array<ClinicCore> = [];
  clinics.map((row: Clinic) => {
    // check for alternateUrl and use that instead
    const scrapingUrl = row.alternateUrl ? row.alternateUrl : row.url;
    if (scrapingUrl && scrapingUrl.includes(keyword)) {
      coreClinicData.push({
        id: row.id,
        url: scrapingUrl,
        key: row.key,
        humanName: row.name,
        address: row.address,
        scraper_config: row.scraper_config,
      });
    }
  });
  return coreClinicData;
}

// Internal-Only request to get all Clinic data
export function getClinics(): Promise<Clinic[]> {
  const requestJson = {
    secret,
  };

  return new Promise<Clinic[]>((resolve, reject) => {
    const protocol = new URL(getClinicsUrl).protocol;
    const agent = protocol === 'http:' ? http : https;
    const request = agent.request(
      getClinicsUrl,
      {
        method: 'POST',
      },
      res => {
        let response = '';
        res.on('data', (chunk: Buffer) => {
          response += chunk;
        });
        res.on('end', () => {
          if (res.statusCode === 200) {
            try {
              const data = JSON.parse(response);
              const clinics = data.data;
              resolve(clinics);
            } catch {
              error(`HTTP error: ${res.statusCode}, data: ${response}`);
            }
          } else {
            reject(error(`HTTP error: ${res.statusCode}, data: ${response}`));
          }
        });
        res.on('error', reject);
      }
    );
    request.on('error', reject);
    request.setHeader('Content-Type', 'application/json');
    request.write(JSON.stringify(requestJson));
    request.end();
  });
}

//returns map of api key => ClinicStatus
export function getClinicStatuses(): Promise<Map<String, ClinicStatus>> {
  return new Promise<Map<String, ClinicStatus>>((resolve, reject) => {
    fetch(getClinicsStatusUrl, {
      timeout: 3000,
    })
      .then(res => res.text())
      .then(json => {
        const data = JSON.parse(json);
        const clinicStatuses = data.data;
        const statusesMap = new Map<string, ClinicStatus>();
        for (const clinicStatus of clinicStatuses) {
          statusesMap.set(clinicStatus.id, clinicStatus);
        }
        resolve(statusesMap);
      })
      .catch(err => reject(err));
  });
}

let cachedStatus: Map<String, ClinicStatus> = new Map<String, ClinicStatus>();
let cachedStatusExpiry = 0;
const cachedStatusTTL = 10000; //10 seconds
export function getClinicStatus(id: String): Promise<ClinicStatus> {
  return new Promise<ClinicStatus>((resolve, reject) => {
    if (cachedStatusExpiry > Date.now()) {
      resolve(cachedStatus.get(id)!);
    } else {
      getClinicStatuses()
        .then(statuses => {
          cachedStatus = statuses;
          cachedStatusExpiry = Date.now() + cachedStatusTTL;
          resolve(statuses.get(id)!);
        })
        .catch(err => reject(err));
    }
  });
}
