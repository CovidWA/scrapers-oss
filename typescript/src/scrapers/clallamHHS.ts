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

// DEPRECATED

import {Scraper, ScrapeResult} from '../scraper';
import {Page, getPage, isVisible} from '../puppeteer';
import {error} from '../logger';
import {Browser} from 'puppeteer-core';
import {Clinic, getClinicsByUrlKeyword} from '../helpers/getClinics';

class ClallamHHSScraper extends Scraper {
  url: string;

  constructor(humanName: string, key: string, url: string) {
    super(humanName, key);
    this.url = url;
  }
  private async checkVaccinationStatus(page: Page) {
    try {
      this.output = await page.content();
      if (
        this.output.match(/is not currently available/) ||
        this.output.match(/is unavailable at this time/) ||
        this.output.match(/vaccination on this date have been filled/)
      ) {
        this.result = ScrapeResult.NO;
        return;
      }
      if (await isVisible(page, 'select')) {
        this.result = ScrapeResult.YES;
        return;
      }
    } catch (err) {
      error(`Error scraping ${this.humanName}: ${err.toString()}`);
    }
    this.result = ScrapeResult.POSSIBLE;
    this.alarm = true;
  }

  async scrape(browser: Browser) {
    const page = await getPage(browser, this.key, this.url);
    await this.checkVaccinationStatus(page);
    await page.close();
  }
}

export function getClallamHHSScrapers(
  clinics: Array<Clinic>
): ClallamHHSScraper[] {
  const coreClinicData = getClinicsByUrlKeyword('cognitoforms', clinics);
  const clallamHHSScraper = [];
  for (const clinic of coreClinicData) {
    clallamHHSScraper.push(
      new ClallamHHSScraper(clinic.humanName, clinic.key, clinic.url)
    );
  }
  return clallamHHSScraper;
}
