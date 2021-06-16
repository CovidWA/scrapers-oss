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

import {Scraper, ScrapeResult} from '../scraper';
import {getPage} from '../puppeteer';
import {error} from '../logger';
import {Browser} from 'puppeteer-core';
import {processMyChartScreen} from './mychart';
import {Clinic, getClinicsByUrlKeyword} from '../helpers/getClinics';

export class CHIFranciscanScraper extends Scraper {
  private url: string;

  constructor(humanName: string, key: string, url: string) {
    super(humanName, key);
    this.url = url;
  }

  async scrape(browser: Browser) {
    const page = await getPage(browser, this.key, this.url);
    try {
      const result = await processMyChartScreen(this.humanName, page);
      this.result = result;
      this.output = await page.content();
      this.alarm = this.result === ScrapeResult.UNKNOWN;
    } catch (err) {
      error(`Error scraping ${this.humanName}: ${err}`);
      this.alarm = true;
      this.result = ScrapeResult.UNKNOWN;
    }
    await page.close();
  }
}

export function getCHIFranciscanScrapers(
  clinics: Array<Clinic>
): CHIFranciscanScraper[] {
  const coreClinicData = getClinicsByUrlKeyword('catholichealth', clinics);
  const scrapers = [];
  for (const clinic of coreClinicData) {
    scrapers.push(
      new CHIFranciscanScraper(clinic.humanName, clinic.key, clinic.url)
    );
  }
  return scrapers;
}
