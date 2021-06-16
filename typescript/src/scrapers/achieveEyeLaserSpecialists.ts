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
import {Page, getPage} from '../puppeteer';
import {error} from '../logger';
import {Browser} from 'puppeteer-core';

// Scraper parameters. Human name is just for local logs; while
// key is being sent to the API server
const humanName = 'Achieve Eye and Laser Specialists';
const key = 'achieve_eye_laser_specialists';
const url = 'https://calendly.com/achieve-vaccine/covid-vaccination-clinic';

// This sample uses Puppeteer but you are free to use any other tech
// to download and analyze the page.
export class AchieveEyeLaserScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  private async checkVaccinationStatus(page: Page) {
    try {
      await page.waitForSelector('.calendar-loader', {hidden: true});

      const noDatesPopup = await page.$('.calendar-no-dates-button');
      if (noDatesPopup) {
        this.result = ScrapeResult.NO;
        return;
      }

      const availableAppointmentTimes = await page.$$eval(
        'tbody.calendar-table tr td button',
        dateSelectors =>
          dateSelectors
            .filter(e => !e.hasAttribute('disabled'))
            .map(e => e.getAttribute('aria-label'))
      );
      this.result =
        availableAppointmentTimes.length > 0
          ? ScrapeResult.YES
          : ScrapeResult.NO;

      return;
    } catch (err) {
      error(`Error scraping ${humanName}: ${err.toString()}`);
    } finally {
      // https://2ality.com/2013/03/try-finally.html - will happen even though there's a return in the try
      this.output = await page.content();
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
