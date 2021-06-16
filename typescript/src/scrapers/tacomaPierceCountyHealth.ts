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
import {Page, getPage, innerHTML} from '../puppeteer';
import {error} from '../logger';
import {Browser} from 'puppeteer-core';

// Scraper parameters. Human name is just for local logs; while
// key is being sent to the API server
const humanName = 'Tacoma-Pierce County Health Department';
const key = 'tpchd_community';
const url =
  'https://www.tpchd.org/healthy-people/diseases/covid-19/covid-19-vaccine-information';

export class TacomaPierceCountryHealthScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  private async checkVaccinationStatus(page: Page) {
    this.output = await page.content();

    try {
      // Need to keep an eye on this - the widget ID could change
      const selector = '#widget_4179_8140_1556 > div';

      // Check for registration closed text
      const html = await innerHTML(page, selector);
      if (html.match(/Registration is closed/)) {
        this.result = ScrapeResult.NO;
        return;
      }
    } catch (err) {
      error(`Error scraping ${humanName}: ${err.toString()}`);
    }

    // Normally, if we reached here, we don't know what's going on
    this.result = ScrapeResult.POSSIBLE;
    this.alarm = true;
  }

  // Overridden method from the base Scraper class
  async scrape(browser: Browser) {
    const page = await getPage(browser, key, url);

    // Perform scraping
    await this.checkVaccinationStatus(page);

    // Close the Chromium page
    await page.close();
  }
}
