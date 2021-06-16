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
import {Page, getPage, isVisible, innerHTML} from '../puppeteer';
import {error} from '../logger';
import {Browser} from 'puppeteer-core';

const humanName = 'Mt. Shuksan Family Medicine and Dermatology';
const key = 'shuksan_family_medicine';
const url = 'https://www.mtshuksanfamilymedicine.com/news-updates/';

export class ShuksanFamilyMedicineScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  private async checkVaccinationStatus(page: Page) {
    this.output = await page.content();

    try {
      // check if the top article is 'Still no covid19 vaccine'
      const selector =
        'body > div > div > div > div > div.container > div > div > article:nth-child(1)';

      const visible = await isVisible(page, selector);
      if (!visible) {
        this.result = ScrapeResult.POSSIBLE;
        this.alarm = true;
      }

      const html = await innerHTML(page, selector);
      if (html.match(/Still no COVID19 vaccine/)) {
        this.result = ScrapeResult.NO;
        return;
      }
      if (html.match(/has still not received any vaccine/)) {
        this.result = ScrapeResult.NO;
        return;
      }
      if (html.match(/vaccine available/)) {
        this.result = ScrapeResult.YES;
        return;
      }
    } catch (err) {
      error(`Error scraping ${humanName}: ${err.toString()}`);
    }
    this.result = ScrapeResult.POSSIBLE;
    this.alarm = true;
  }

  async scrape(browser: Browser) {
    const page = await getPage(browser, key, url);
    await this.checkVaccinationStatus(page);
    await page.close();
  }
}
