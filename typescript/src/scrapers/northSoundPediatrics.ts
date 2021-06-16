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
import {log, error} from '../logger';
import {Browser, ElementHandle} from 'puppeteer-core';

// Scraper parameters. Human name is just for local logs; while
// key is being sent to the API server
const humanName = 'North Sound Pediatrics scraper';
const key = 'NorthSoundPediatrics';
const url = 'https://northsoundpediatrics.as.me/schedule.php';

// This sample uses Puppeteer but you are free to use any other tech
// to download and analyze the page.
export class NorthSoundPediatricsScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  private async checkVaccinationStatus(page: Page) {
    this.output = await page.content();

    try {
      await page.waitForSelector('div#page-loading', {hidden: true});

      const [appointmentTypeButtons, calendarPicker] = await Promise.all([
        page.$$('div.pick-appointment-pane div.select-item-box'),
        page.$('div.choose-date-time'),
      ]);

      let foundAtLeastOneAppointmentSlot = false;
      for (const button of appointmentTypeButtons) {
        const optionName = await button.$eval(
          'label > span.appointment-type-name',
          e => e.innerHTML
        );
        log(
          `Looking at ${optionName} calendar for appointments up to 6 months in the future.`
        );

        // this hides the other buttons
        await button.click();

        if (await this.hasAvailability(page, calendarPicker!)) {
          foundAtLeastOneAppointmentSlot = true;
          break;
        }

        // this redisplays the other buttons
        await button.click();
        await page.waitForTimeout(1000);
      }

      this.output = await page.content();
      this.result = foundAtLeastOneAppointmentSlot
        ? ScrapeResult.YES
        : ScrapeResult.NO;
      return;
    } catch (err) {
      error(`Error scraping ${humanName}: ${err.toString()}`);
    }

    // Normally, if we reached here, we don't know what's going on
    this.result = ScrapeResult.POSSIBLE;
    this.alarm = true;
  }

  async hasAvailability(page: Page, calendarPicker: ElementHandle) {
    for (let monthAheadCount = 0; monthAheadCount < 6; monthAheadCount++) {
      await page.waitForSelector(
        'div.choose-date-time > div.choose-date > div.loading-container',
        {hidden: true}
      );
      const [currentMonth, nextMonthButton] = await Promise.all([
        calendarPicker.$eval(
          '.calendarHeading option[selected]',
          e => e.innerHTML
        ),
        calendarPicker.$('.calendarHeading [data-qa="next-month-button"]'),
      ]);
      log(`Looking at ${currentMonth}`);

      const noDatesAvailableIndicator = await calendarPicker.$(
        'div.choose-date > .calendar .no-dates-available'
      );
      if (!noDatesAvailableIndicator) {
        return true;
      }

      await nextMonthButton!.click();
    }

    return false;
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
