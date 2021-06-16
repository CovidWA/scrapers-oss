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
import {Page, getPage, waitAndClick} from '../puppeteer';
import {error} from '../logger';
import {Browser} from 'puppeteer-core';

const humanName = 'ARCpoint Labs';
const key = 'arcpoint_labs_ts';
const url = 'https://covidvaccinemarysville.timetap.com/#/';

export class ARCpointLabsScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  private async checkVaccinationStatus(page: Page) {
    try {
      // Await page and start clicking through.
      // Click NEXT button to get to Locations
      try {
        await waitAndClick(page, '#nextBtn > span');
      } catch (err) {
        error(
          `Something changed, could not click 1st NEXT button for ${humanName}: ${err.toString()}`
        );
        this.result = ScrapeResult.POSSIBLE;
        this.alarm = true;
        return;
      }

      // See if we get the No Locations message
      try {
        const noLocationsSelector =
          '#schedulerBox > locations-panel > mat-card-content > form > div';
        // const yesLocationsSelector =
        //   '#schedulerBox > locations-panel > mat-card-content > form > mat-list > mat-list-item';

        // N.B. More often than not, MPFH does not have availability,
        // so check for No Locations, rather than checking for locations
        // found selector:
        //     '#schedulerBox > locations-panel > mat-card-content > form > mat-list > mat-list-item';
        // Takes a while to timeout either way.
        await page.waitForSelector(noLocationsSelector, {timeout: 10000});
        const element = await page.$(noLocationsSelector);
        const txt = await page.evaluate(el => el.textContent, element);

        if (txt.match(/There are no locations available/)) {
          console.log('No locations available');
          this.result = ScrapeResult.NO;
          return;
        }
      } catch (err) {
        console.log(
          `Continuing, since we didn't find the No Locations msg for ${humanName}`
        );
      }

      // Click NEXT button to get to Service page.
      try {
        await waitAndClick(page, '#nextBtn > span');
      } catch (err) {
        error(
          `Something changed, could not click 2nd NEXT for ${humanName}: ${err.toString()}`
        );
        this.result = ScrapeResult.POSSIBLE;
        this.alarm = true;
        return;
      }

      // On Service page. If more than 1 service (e.g., Moderna 1 v. Moderna 2),
      // will need to click on a service link. If there is only 1 service, I think
      // it works to just click the NEXT button.
      await page.waitForSelector('[id^=rowFor]', {timeout: 5000});
      let totalAvailability = 0;
      try {
        // Find the Service page link(s) using the CSS selector for id that starts with
        // "rowFor". The reasonId (currently 607236 for Moderna 1, 607237 for
        // Moderna 2) is appended to "rowFor". Example:
        // 'mat-list-item#rowFor607237.mat-list-item.mat-list-item-no-avatar'
        // Right now, just click on the first link and bring up its screening question.
        // TODO: Click on all links to gather their separate availability counts for LIMITED.
        // Right now, we're just checking the first link.
        // TODO: Sometimes there isn't a screening question (e.g., Moderna 2). Right now,
        // just continue on after the timeout.
        //
        // N.B. Sometimes you don't have to click on a Service, but after looking at
        // kphd.timetap.com in Chrome, it appears that they still have a 'forFor' in
        // that case, so this code should still be okay even if there is only 1 service
        // and you could probably judt click NEXT.

        const serviceLinks = await page.$$('[id^=rowFor]');
        // console.log(serviceLinks);
        await serviceLinks[0].click();
      } catch (err) {
        error(
          `Something changed, could not click on SERVICE for ${humanName}: ${err.toString()}`
        );
        this.result = ScrapeResult.POSSIBLE;
        this.alarm = true;
        return;
      }

      await page.waitForTimeout(2000);

      // Screening question, click NO to pass.
      try {
        await waitAndClick(page, '#screeningQuestionPassBtn');
      } catch (err) {
        console.log(
          `Continuing although there was no screening button to click for ${humanName}`
        );
      }
      // Now we should be at a scheduling calendar that either
      // has days that can be scheduled or gives the "no times available"
      // message.

      this.output = await page.content();

      // Check for time slots to schedule.
      try {
        const timeSelector = 'div.day.dayBox.dayHasAvailability';
        //'#calendarTimes > div:nth-child(2) > span > div:nth-child(1)';
        await page.waitForSelector(timeSelector);
        // console.log('There are times to schedule');
        const timeSlots = await page.$$('div.row.time-list');
        // console.log(timeSlots.length);
        totalAvailability += timeSlots.length;
        if (totalAvailability > this.limitedThreshold) {
          this.result = ScrapeResult.YES;
        } else {
          this.result = ScrapeResult.LIMITED;
        }
        return;
      } catch (err) {
        console.log(
          `timeSelector not found for ${humanName}: ${err.toString()}`
        );
      }

      // Check for "All appointment times are currently reserved".
      try {
        const allReservedSelector =
          '#schedulerBox > time-panel > mat-card-content > div';
        await page.waitForSelector(allReservedSelector, {timeout: 10000});
        console.log('All appointment times are currently reserved');
        this.result = ScrapeResult.NO;
        return;
      } catch (err) {
        console.log(
          `allReservedSelector not found for ${humanName}: ${err.toString()}`
        );
      }

      // Check for another type of error text indicating no appointments
      // available. Not sure whether we will run into this one. If it never
      // happens, remove this code.
      try {
        const noTimesSelector =
          '#calendarTimes > div:nth-child(2) > span > div';
        await page.waitForSelector(noTimesSelector);
        console.log('Not seeing times to select', {timeout: 10000});
        this.result = ScrapeResult.NO;
        return;
      } catch (err) {
        console.log(
          `noTimesSelector not found for ${humanName}: ${err.toString()}`
        );
      }

      // Geez, maybe something else changed? The result is probably already set to
      // POSSIBLE, but just in case, set it to POSSIBLE here.
      this.result = ScrapeResult.POSSIBLE;
      this.alarm = true;
      return;
    } catch (err) {
      error(`Error scraping ${humanName}: ${err.toString()}`);
      this.output = await page.content();
      this.alarm = true;
      this.result = ScrapeResult.UNKNOWN;
    }
  }

  async scrape(browser: Browser) {
    const page = await getPage(browser, key, url);
    this.result = ScrapeResult.POSSIBLE;
    await page.content();

    await this.checkVaccinationStatus(page);

    await page.close();
  }
}
