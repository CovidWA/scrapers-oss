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
import {getPage, isVisible, Page, sleep} from '../puppeteer';
import {error, log} from '../logger';
import {Browser} from 'puppeteer-core';
// import {processMyChartScreen} from './mychart';
import {Clinic, getClinicsByUrlKeyword} from '../helpers/getClinics';

const infoUrl = 'https://www.multicare.org/covid19/vaccine/';

export class MulticareDirectScraper extends Scraper {
  private url: string;

  private static async processMychartScreen(humanName: string, page: Page) {
    try {
      // Loading screen
      const loadingSelector = 'div.loadingmessage';
      const firstApptSelector = 'a.slotdetailaction';
      const noApptSelector = 'div.errormessage';
      for (let second = 0; second < 30; ++second) {
        await sleep(1000);
        const loadingVisible = await isVisible(page, loadingSelector, true);
        if (loadingVisible) {
          log('Loading screen. Waiting...');
          continue;
        }
        const firstApptVisible = await isVisible(page, firstApptSelector, true);
        if (firstApptVisible) {
          log('Found first appointment!');
          return ScrapeResult.YES;
        }
        const noApptVisible = await isVisible(page, noApptSelector, true);
        if (noApptVisible) {
          log('Found no appointments message.');
          return ScrapeResult.NO;
        }
        log('No known element found, waiting...');
      }
      return ScrapeResult.POSSIBLE;
    } catch (err) {
      error(`Error scraping ${humanName}: ${err.toString()}`);
      return ScrapeResult.UNKNOWN;
    }
  }

  constructor(humanName: string, key: string, url: string) {
    super(humanName, key);
    this.url = url;
  }

  async scrape(browser: Browser) {
    // Make sure the main page has a jotform.
    // If I actually knew Typescript, I would figure
    // out a smart way to only do this check one time,
    // not for each scrape. But I need browser, and we've
    // actually got one here. :)
    const initialPage = await getPage(
      browser,
      'Multicare vaccine info',
      infoUrl
    );
    this.output = await initialPage.content();
    await initialPage.close();

    if (!this.output.includes('multicare.jotform.com')) {
      // The main page does not have a link to MyChart. Responding NO.
      log('No link to the MyChart signup page found.');
      this.result = ScrapeResult.NO;
      return;
    }

    // Process the MyChart page.
    const page = await getPage(browser, this.key, this.url);
    try {
      const result = await MulticareDirectScraper.processMychartScreen(
        this.humanName,
        page
      );
      this.result = result;
      this.output = await page.content();
      this.alarm = this.result === ScrapeResult.UNKNOWN;
    } catch (err) {
      error(`Error scraping ${this.humanName}: ${err}`);
      this.alarm = true;
      this.result = ScrapeResult.UNKNOWN;
    }
    await page.close();
  }
}

export function getMulticareDirectScrapers(
  clinics: Array<Clinic>
): MulticareDirectScraper[] {
  const coreClinicData = getClinicsByUrlKeyword('multicare', clinics);
  const scrapers = [];
  for (const clinic of coreClinicData) {
    // console.log(`${clinic.humanName} got url = ${clinic.url}`);
    scrapers.push(
      new MulticareDirectScraper(clinic.humanName, clinic.key, clinic.url)
    );
  }

  return scrapers;
}
