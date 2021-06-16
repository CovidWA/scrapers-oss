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
import {
  Page,
  isVisible,
  sleep,
  getPage,
  innerHTML,
  waitAndClick,
} from '../puppeteer';
import {log, error} from '../logger';
import {Browser} from 'puppeteer-core';

// Scraper parameters. Human name is just for local logs; while
// key is being sent to the API server
const humanName = 'Horizon Pharmacy';
const key = 'horizon_pharmacy';
const url = 'https://horizonpharmacyvaccineappointment.as.me/schedule.php';

export class HorizonPharmacyScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  // controls how many months past the current month
  // the scraper will look at for appointments
  monthsOut = 3;

  // checks an individual month for failure case
  // only one month is displayed at a time in the
  // calendar viewer, so this method is rerun every time
  // the calendar viewer reloads (when the next month button is clicked)
  private async checkMonth(page: Page) {
    // selector for no times available element
    const selector = '#no-times-available-message';

    try {
      // attempt to wait for the selector to appear
      await page.waitForSelector(selector, {
        visible: true,
        timeout: 10000,
      });

      await sleep(1000);
    } catch (err) {
      // if we can't find the selector, return POSSIBLE
      // scraper does not yet know how to handle non-failure
      // cases
      log('Cannot find element that says No appointments');
      this.output = await page.content();
      return ScrapeResult.POSSIBLE;
    }

    this.output = await page.content();

    // ensure element is visible, otherwise return POSSIBLE
    const visible = await isVisible(page, selector);
    if (!visible) {
      log('No appointments element is invisible');
      return ScrapeResult.POSSIBLE;
    }

    // match failure case against element's inner text
    // to ensure we have really hit the failure case
    const html = await innerHTML(page, selector);
    if (html.match(/No times are available/)) {
      return ScrapeResult.NO;
    }

    return ScrapeResult.POSSIBLE;
  }

  private async checkVaccinationStatus(page: Page) {
    // selector for button which loads next
    // month calendar view
    const calendarNextSelector = '.calendar-next';

    // represents the number of times we have loaded
    // the NEXT month's calendar view
    let nextMonthsChecked = 0;

    try {
      // search for failure case for current month calendar view,
      // and `this.monthsOut` months after the current
      // month
      while (nextMonthsChecked <= this.monthsOut) {
        this.result = await this.checkMonth(page);
        if (this.result === ScrapeResult.POSSIBLE) {
          this.alarm = true;
          return;
        }

        // click button to move to next month's calendar view
        await waitAndClick(page, calendarNextSelector);
        nextMonthsChecked++;
      }
    } catch (err) {
      this.result = ScrapeResult.POSSIBLE;
      this.alarm = true;
      error(`Error scraping ${humanName}: ${err.toString()}`);
    }
  }

  async scrape(browser: Browser) {
    const page = await getPage(browser, key, url);
    await this.checkVaccinationStatus(page);
    await page.close();
  }
}
