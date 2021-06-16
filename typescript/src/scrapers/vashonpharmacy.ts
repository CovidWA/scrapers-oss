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
import {Page, getPage, innerHTML, isVisible} from '../puppeteer';
import {error} from '../logger';
import {Browser} from 'puppeteer-core';

const humanName = 'Vashon Pharmacy';
const key = 'vashon_pharmacy';
const url = 'https://vashonpharmacy.com/covid/';

export class VashonPharmacyScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  private async checkVaccinationStatus(page: Page) {
    this.output = await page.content();

    try {
      const apptSelector =
        '#content > div > div.general_content > div.main_content > div.wp-block-cover.has-background-dim-60.has-background-dim > div > h1';

      const visible = await isVisible(page, apptSelector);
      if (!visible) {
        this.result = ScrapeResult.POSSIBLE;
        this.alarm = true;
      }

      const html = await innerHTML(page, apptSelector);
      if (html.match(/Vaccine Registration Status: Closed/)) {
        this.result = ScrapeResult.NO;
        return;
      }
      if (html.match(/Vaccine Registration Status: Open/)) {
        // They won't let us fill in the form to see if appts are actually available
        this.result = ScrapeResult.POSSIBLE;
        return;
      }
    } catch (err) {
      error(`Error scraping ${humanName}: ${err.toString()}`);
    }

    this.result = ScrapeResult.POSSIBLE;
    this.alarm = true;
    return;
  }

  async scrape(browser: Browser) {
    const page = await getPage(browser, key, url);
    await this.checkVaccinationStatus(page);
    await page.close();
  }
}
