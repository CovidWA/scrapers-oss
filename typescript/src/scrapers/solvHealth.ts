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
import {Clinic, getClinicsByUrlKeyword} from '../helpers/getClinics';
import {Browser} from 'puppeteer-core';

// Generic SolvHealth Url Scraper
class SolvHealthScraper extends Scraper {
  url: string;

  constructor(humanName: string, key: string, url: string) {
    super(humanName, key);
    this.url = url;
  }

  private async checkVaccinationStatus(page: Page) {
    // Save the page content to the server if needed
    this.result = ScrapeResult.UNKNOWN;
    // Some pages have a preScreenModal and some don't
    try {
      await page.click(
        'div[class^="PreScreenerModal__StyledFooter"] > button[text="Yes"]'
      );
    } catch {
      // do nothing some pages don't have it.
    }
    await sleep(1000);
    this.output = await page.content();
    if (this.output.match(/Due to high demand all of our appointments/)) {
      this.result = ScrapeResult.NO;
    } else {
      try {
        const [openCalButton] = await page.$x(
          "//button[contains(., 'Find Next Available Visit')]"
        );
        await openCalButton.click();
        await sleep(2000);
        const htmlForCalendar = await innerHTML(
          page,
          'form > div[class^="TimeSelectorModal__Container"]'
        );
        if (
          htmlForCalendar.match(/All online booking slots are full/) ||
          htmlForCalendar.match(/No Visit Times Left/)
        ) {
          this.result = ScrapeResult.NO;
        } else {
          this.result = ScrapeResult.POSSIBLE;
        }
      } catch (err) {
        error(`Error scraping ${this.humanName}: ${err.toString()}`);
        this.output = await page.content();
        this.alarm = true;
        this.result = ScrapeResult.POSSIBLE;
      }
    }
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

export function getSolvHealthScrapers(
  clinics: Array<Clinic>
): SolvHealthScraper[] {
  const coreClinicData = getClinicsByUrlKeyword('solv', clinics);

  const solvHealthScrapers = [];
  for (const clinic of coreClinicData) {
    solvHealthScrapers.push(
      new SolvHealthScraper(clinic.humanName, clinic.key, clinic.url)
    );
  }
  return solvHealthScrapers;
}
