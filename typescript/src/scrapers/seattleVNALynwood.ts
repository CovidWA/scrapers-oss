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
import {Page, getPage, innerHTML, sleep} from '../puppeteer';
import {error} from '../logger';
import {Browser} from 'puppeteer-core';

const humanName = 'Seattle Visiting Nurse Association';
const key = 'seattle_vna_lynwood';
const url =
  'https://schedule.seattlevna.com/home/fbb4a1ad-7e4b-eb11-a813-000d3a3033d3';

export class SeattleVNALynwoodScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  private async checkVaccinationStatus(page: Page) {
    await sleep(3000);
    this.output = await page.content();

    try {
      const html = await innerHTML(page, 'section:nth-child(2) > div');
      if (html.match(/We apologize but we currently have no COVID-19/)) {
        this.result = ScrapeResult.NO;
        return;
      }
    } catch (err) {
      error(`Error scraping ${humanName}: ${err.toString()}`);
      this.output = await page.content();
      this.alarm = true;
      this.result = ScrapeResult.POSSIBLE;
    }
    // Normally, if we reached here, we don't know what's going on
    this.result = ScrapeResult.POSSIBLE;
    this.alarm = true;
  }

  async scrape(browser: Browser) {
    const page = await getPage(browser, key, url);
    await this.checkVaccinationStatus(page);
    await page.close();
  }
}
