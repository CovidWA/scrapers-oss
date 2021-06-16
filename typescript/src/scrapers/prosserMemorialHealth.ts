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
import {Page, isVisible, sleep, getPage, waitAndClick} from '../puppeteer';
import {log, error} from '../logger';
import {Browser} from 'puppeteer-core';

// Scraper parameters. Human name is just for local logs; while
// key is being sent to the API server
const humanName = 'Prosser Memorial Health';
const key = 'ProsserMemorialHealthScraper';
const url = 'https://www.prosserhealth.org/book-online';

export class ProsserMemorialHealthScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  private async checkVaccinationStatus(page: Page) {
    try {
      // Question 1: yes
      await waitAndClick(page, '#comp-kjw046v8 > button > div', 10000, 1000);
      // Question 2: no
      await waitAndClick(page, '#comp-kjw046ur > button > div', 10000, 1000);
      // Question 3: no
      await waitAndClick(page, '#comp-kjw046ur > button > div', 10000, 1000);
      // Get the text that says if available or not
      await sleep(1000);

      this.output = await page.content();
      const selector =
        '#TPASection_k041o4d2 > div > div > div.EmptyStateView1470909390__root > div';
      try {
        await isVisible(page, selector);
        this.result = ScrapeResult.NO;
        return;
      } catch (err) {
        log('Cannot find element that says No appointments');
        this.output = await page.content();
        this.result = ScrapeResult.POSSIBLE;
        this.alarm = true;
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
