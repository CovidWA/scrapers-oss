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
import {Page, getPage, innerHTML, isVisible, sleep} from '../puppeteer';
import {error} from '../logger';
import {Browser} from 'puppeteer-core';

// Scraper parameters. Human name is just for local logs; while
// key is being sent to the API server
const humanName = 'Example scraper';
const key = 'Acme';
const url = 'http://example.com';

// This sample uses Puppeteer but you are free to use any other tech
// to download and analyze the page.
export class ExampleScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  private async checkVaccinationStatus(page: Page) {
    // Save the page content to the server if needed
    this.output = await page.content();

    // Set this.result to one of:
    // - ScrapeResult.YES if open appointments are found
    // - ScrapeResult.NO if there are no appointments
    // - ScrapeResult.POSSIBLE if the scraper is unsure

    try {
      // In Chrome dev tools, pick an element and use right click -> Copy -> Selector
      const selector = 'body > div > p:nth-child(2)';

      // Sometimes the element we need exists but is not visible
      const visible = await isVisible(page, selector);
      if (!visible) {
        // Need to send a PagerDuty alert if something is wrong?
        this.result = ScrapeResult.POSSIBLE;
        this.alarm = true;
        return;
      }

      // Sometimes we need to emulate a mouse click
      await page.click(selector);

      // Sometimes we need to wait for an element to appear
      await page.waitForSelector(selector, {visible: true});

      // ...or just sleep for a second
      await sleep(1000);

      // Sometimes we need to check for the specific text
      const html = await innerHTML(page, selector);
      if (html.match(/domain is for use in illustrative examples/)) {
        this.result = ScrapeResult.NO;
        return;
      }
    } catch (err) {
      error(`Error scraping ${humanName}: ${err.toString()}`);
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
