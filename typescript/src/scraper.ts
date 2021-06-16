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

import * as http from 'http';
import * as https from 'https';
import * as FormData from 'form-data';
import * as config from '../config.json';
import {Browser} from 'puppeteer-core';
import * as AWS from 'aws-sdk';

export enum ScrapeResult {
  NO = 'No',
  YES = 'Yes',
  POSSIBLE = 'Possible',
  UNKNOWN = 'Unknown',
  LIMITED = 'Limited',
  API_FAILED = 'APIFailed',
}

export enum VaccineType {
  PFIZER = 'pfizer',
  MODERNA = 'moderna',
  JOHNSON = 'johnson',
}

export interface RequestData {
  key: string;
  status: ScrapeResult;
  secret: string;
  alarm?: boolean;
  content_url?: string;
  scraperTags?: string[];
}

export abstract class Scraper {
  protected humanName: string;
  protected key: string;
  protected result: ScrapeResult = ScrapeResult.UNKNOWN;
  protected output = '';
  protected alarm = false;
  protected doses?: string[];
  protected types?: string[];
  protected nearestDate?: string;
  protected skipBackendReporting = false;
  protected needsBrowserBool = true;
  protected limitedThreshold = 5;
  private content_url?: string;

  constructor(humanName: string, key: string) {
    this.humanName = humanName;
    this.key = key;
  }

  getHumanName() {
    return this.humanName;
  }

  getResult() {
    return this.result;
  }

  getKey() {
    return this.key;
  }

  needsBrowser() {
    return this.needsBrowserBool;
  }

  abstract scrape(browser?: Browser): void;

  async report() {
    if (this.skipBackendReporting) {
      return 'skipping backend call';
    }
    if (!process.env.AWS_LAMBDA) {
      // NO LOGGING FROM A LOCAL COMPUTER
      return 'Mock response on a local computer. DEBUG is turned on.';
    }

    try {
      if (this.alarm && this.output.length > 0) {
        this.content_url = await this.getS3HtmlUrl();
      }
    } catch (err) {
      console.log(`Unable to get content url ${err}`);
    }
    const response = await this.reportResult();
    return response;
  }

  private async fileUrlWithinXtime(hours: number): Promise<string | undefined> {
    const s3 = new AWS.S3();

    const response = await s3
      .listObjectsV2({
        Bucket: 'covidwa-scrapers-html',
        Prefix: this.key,
      })
      .promise();
    const s3Objects = response.Contents || [];

    const maxTime = new Date();
    maxTime.setHours(maxTime.getHours() - hours);
    if (s3Objects && s3Objects.length > 0) {
      // the sdk makes all fields options so making this any
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      s3Objects.sort((a: any, b: any) => {
        return Date.parse(a.LastModified) - Date.parse(b.LastModified);
      });
      const s3ObjectByDate = s3Objects.reverse();
      if (
        s3ObjectByDate[0].LastModified &&
        // file is newer than 2 hours ago
        s3ObjectByDate[0].LastModified > maxTime
      ) {
        return `https://covidwa-scrapers-html.s3.us-west-2.amazonaws.com/${s3ObjectByDate[0].Key}`;
      }
    }
    return;
  }

  private async getS3HtmlUrl() {
    const now = new Date().getTime();
    const filename = `${this.key}-${now}.html`;
    const s3 = new AWS.S3();
    const bucket = 'covidwa-scrapers-html';
    const previousUrl = await this.fileUrlWithinXtime(2);
    if (!previousUrl) {
      const result = await s3
        .upload({
          Bucket: bucket,
          Key: filename,
          Body: this.output,
          ContentType: 'text/html',
          ACL: 'public-read',
        })
        .promise();
      console.log(`returning current url ${result.Location}`);
      return result.Location;
    }
    console.log(`returning previous url ${previousUrl}`);
    return previousUrl;
  }

  protected reportResult() {
    const requestData: RequestData = {
      key: this.key,
      status: this.result,
      secret: config.secret,
    };

    if (this.alarm) {
      requestData.alarm = this.alarm;
      requestData.content_url = this.content_url;
    }

    if (this.types && this.types.length > 0) {
      requestData.scraperTags = this.types;
    }

    return this.sendScraperResult(requestData);
  }

  protected async sendScraperResult(requestData: RequestData) {
    return new Promise<string>((resolve, reject) => {
      const form = new FormData();
      for (const [key, value] of Object.entries(requestData)) {
        form.append(
          key,
          Array.isArray(value) ? JSON.stringify(value) : value.toString()
        );
      }

      const onResponse = (res: http.IncomingMessage) => {
        let response = '';
        res.on('data', (chunk: Buffer) => {
          response += chunk.toString();
        });
        res.on('end', () => {
          if (res.statusCode === 200) {
            resolve(response);
          } else {
            reject(
              new Error(`HTTP error: ${res.statusCode}, data: ${response}`)
            );
          }
        });
        res.on('error', reject);
      };

      const apiUrl = config.reportServerUrl;
      const protocol = new URL(apiUrl).protocol;
      const agent = protocol === 'http:' ? http : https;
      const request = agent.request(apiUrl, {
        method: 'POST',
        headers: form.getHeaders(),
      });
      form.pipe(request);
      request.on('response', onResponse);
      request.on('error', reject);
      request.end();
    });
  }
}
