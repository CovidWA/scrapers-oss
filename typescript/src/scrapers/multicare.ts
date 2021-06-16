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
import {Page, getPage, sleep, isVisible} from '../puppeteer';
import {log, error} from '../logger';
import {Browser} from 'puppeteer-core';

const infoUrl = 'https://www.multicare.org/covid19/vaccine/';
const myChartUrl =
  'https://mychart.multicare.org/mymulticare/SignupAndSchedule/EmbeddedSchedule?id=93281,91335,91314,92498&vt=18398&dept=10114,10093,10096,100125&view=plain&public=1';

// This is MyChart but listing multiple departments at the same page.
// We'll scrape once for all departments and store the results for
// subsequent scraper invocations for other departments.

export class MulticareScraper extends Scraper {
  private static hasAppointments: {[name: string]: boolean} = {};
  private static isBroken = false;
  private static processed = false;
  private static output = '';

  private static async processMychartPage(page: Page) {
    // Note: this logic differs from the one in mychart.ts.

    let slotFound = false;
    try {
      // Loading screen
      const loadingSelector = 'div.loadingmessage';
      const slotSelector = 'a.slotdetailaction';
      const noApptSelector = 'div.errormessage';
      for (let second = 0; second < 30; ++second) {
        await sleep(1000);
        const loadingVisible = await isVisible(page, loadingSelector, true);
        if (loadingVisible) {
          log('Loading screen. Waiting...');
          continue;
        }
        const firstApptVisible = await isVisible(page, slotSelector, true);
        if (firstApptVisible) {
          log('Found a slot!');
          slotFound = true;
          break;
        }
        const noApptVisible = await isVisible(page, noApptSelector, true);
        if (noApptVisible) {
          log('Found no appointments message.');
          break;
        }
        log('No known element found, waiting...');
      }
    } catch (err) {
      error(`Error scraping MultiCare MyChart (1): ${err.toString()}`);
      MulticareScraper.isBroken = true;
    }

    MulticareScraper.processed = true;

    if (!slotFound) {
      return;
    }

    try {
      const divs = await page.$$('div.scrollTableWrapper div');
      let name = '';
      let date = '';
      let time = '';
      for (const div of divs) {
        // TODO: date string: 'class="header medium extraWide"'
        const className = await div.evaluate((d: HTMLElement) => d.className);
        const innerText = await div.evaluate((d: HTMLElement) => d.innerText);
        if (className === 'header medium extraWide') {
          date = innerText;
          name = '';
          time = '';
          log(`Found date: ${date}`);
        } else if (className === 'cardline name heading') {
          name = innerText.replace(/\n.*/, '');
          log(`Found name: ${name}`);
        } else if (className.includes('slotslist')) {
          const anchors = await div.$$('a.slotdetailaction');
          if (anchors.length > 0) {
            const anchor = anchors[0];
            time = await anchor.evaluate((a: HTMLElement) => a.innerText);
            time = time.replace(/(^.*?(?:AM|PM)).*/, '$1');
            log(`Found time: ${time}`);
            if (
              name &&
              date &&
              time &&
              !(name in MulticareScraper.hasAppointments)
            ) {
              log(
                `Found an earliest appointment for ${name} at ${date} ${time}.`
              );
              MulticareScraper.hasAppointments[name] = true;
            }
          }
        }
      }
    } catch (err) {
      error(`Error scraping MultiCare MyChart (2): ${err.toString()}`);
      MulticareScraper.isBroken = true;
    }
  }

  private static async scrapeAndSave(browser: Browser) {
    const initialPage = await getPage(
      browser,
      'Multicare vaccine info',
      infoUrl
    );
    MulticareScraper.output = await initialPage.content();
    await initialPage.close();
    if (!MulticareScraper.output.includes('multicare.jotform.com')) {
      // The main page does not have a link to MyChart. Responding NO.
      log('No link to the MyChart signup page found.');
      MulticareScraper.processed = true;
      return;
    }

    const page = await getPage(browser, 'Multicare general', myChartUrl);
    MulticareScraper.output = await page.content();
    await MulticareScraper.processMychartPage(page);
    await page.close();
  }

  constructor(humanName: string, key: string) {
    super(humanName, key);
  }

  async scrape(browser: Browser) {
    if (!MulticareScraper.processed) {
      await MulticareScraper.scrapeAndSave(browser);
    }

    this.output = MulticareScraper.output;
    if (MulticareScraper.isBroken) {
      this.result = ScrapeResult.POSSIBLE;
      this.alarm = true;
      return;
    }

    if (MulticareScraper.hasAppointments[this.humanName]) {
      this.result = ScrapeResult.YES;
      log(`Reporting YES for ${this.humanName}`);
    } else {
      this.result = ScrapeResult.NO;
      log(`Reporting NO for ${this.humanName}`);
    }
  }
}

export function getMulticareScrapers(): MulticareScraper[] {
  const scrapers: MulticareScraper[] = [];

  // We manually add Multicare scrapers for locations we are aware of.
  // Note: humanName must be exactly the same as in their MyChart page.
  scrapers.push(
    new MulticareScraper('Auburn Offsite Vaccine', 'multicare_auburn')
  );
  scrapers.push(
    new MulticareScraper('Good Samaritan Vaccine', 'multicare_puyallup')
  );
  scrapers.push(
    new MulticareScraper('Tacoma General Vaccine', 'multicare_tacoma')
  );
  scrapers.push(
    new MulticareScraper('RWC Deaconess Vaccine', 'multicare_spokane')
  );

  return scrapers;
}
