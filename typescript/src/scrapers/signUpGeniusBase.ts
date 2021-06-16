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
import {Page, getPage} from '../puppeteer';
import {error} from '../logger';
import {Browser} from 'puppeteer-core';

export abstract class SignUpGeniusScraper extends Scraper {
  protected url = '';

  private async checkVaccinationStatus(page: Page) {
    this.output = await page.content();
    try {
      if (this.output.match(/The Sign Up Was Not Found/)) {
        this.result = ScrapeResult.NO;
        this.alarm = true;
        return;
      }
      await page.waitForSelector('.SUGtableouter', {visible: true});

      const appointmentSignUpButtons = await page.$$eval(
        '.SUGbuttonContainer',
        elems => Promise.all(elems.map(e => e.innerHTML))
      );

      this.result =
        appointmentSignUpButtons.length > 0
          ? ScrapeResult.YES
          : ScrapeResult.NO;
      return;
    } catch (err) {
      error(`Error scraping ${this.humanName}: ${err.toString()}`);
    }

    // Normally, if we reached here, we don't know what's going on
    this.output = await page.content();
    this.result = ScrapeResult.POSSIBLE;
    this.alarm = true;
  }

  // Overridden method from the base Scraper class
  async scrape(browser: Browser) {
    const page = await getPage(browser, this.key, this.url);

    // Perform scraping
    await this.checkVaccinationStatus(page);

    // Close the Chromium page
    await page.close();
  }
}
