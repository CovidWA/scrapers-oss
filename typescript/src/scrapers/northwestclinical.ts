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
import {getPage, innerHTML, isVisible, Page, sleep} from '../puppeteer';
import {error} from '../logger';
import {Browser} from 'puppeteer-core';

const humanName = 'Northwest Clinical Research Center';
const key = 'northwest_clinical';
const url = 'https://calendly.com/nwcrc_covidvaccine/covid-19-vaccination';

export class NorthwestClinicalScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  private async checkVaccinationStatus(page: Page) {
    this.output = await page.content();

    try {
      const apptSelector =
        '#page-region > div > div > div > div._11234___MainPanel__cls1 > div > div > div > div > div.calendar._1usD3___dates-StyledCalendar__cls1.is-locked > div.calendar-table-wrapper > div.calendar-no-dates > div > div';
      const validDateSelector =
        '_2lqEN___day-Button__cls1 U5hxE___day-Button__bookable _1Qg-r___BareButton__cls1 _2zIir___index-UnstyledButton__cls1';
      const invalidCalendarContent = 'This Calendly URL is not valid.';

      const headerContents = await innerHTML(page, 'h2');
      if (
        headerContents &&
        headerContents.indexOf(invalidCalendarContent) > -1
      ) {
        console.log(
          '[EXTERNAL ERROR] NW Clinical Calendly URL is not valid. No scheduling system available.'
        );
        this.result = ScrapeResult.NO;
        this.alarm = true;
        return;
      }

      const visible = await isVisible(page, apptSelector);
      if (!visible) {
        this.result = ScrapeResult.POSSIBLE;
        this.alarm = true;
      }

      const html = await innerHTML(page, apptSelector);
      if (html.match(/No times in/)) {
        this.result = ScrapeResult.NO;
        return;
      }

      const validDateVisible = await isVisible(page, validDateSelector);
      if (validDateVisible) {
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
    await sleep(1000);
    await this.checkVaccinationStatus(page);
    await page.close();
  }
}
