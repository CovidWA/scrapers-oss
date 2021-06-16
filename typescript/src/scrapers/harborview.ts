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
import {Page, getPage, isVisible, innerHTML} from '../puppeteer';
import {error} from '../logger';
import {Browser} from 'puppeteer-core';

const humanName = 'Harborview Medical Center';
const key = 'harborview';
const url = 'https://www.uwmedicine.org/coronavirus/vaccine';

export class HarborViewScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  private async checkVaccinationStatus(page: Page) {
    this.output = await page.content();

    try {
      const noApptSelector =
        '#main-container > article > div.field.field--name-field-uwm-sections.field--type-entity-reference-revisions.field--label-hidden.field__items > section.uwm-section.paragraph--view-mode--default.paragraph--id--97346.uwm-section--heading-hidden > div > div > div > div > div > h2:nth-child(6)';

      const visible = await isVisible(page, noApptSelector);
      if (!visible) {
        this.result = ScrapeResult.POSSIBLE;
        this.alarm = true;
        return;
      }

      const html = await innerHTML(page, noApptSelector);
      if (
        html.match(
          /First-dose vaccination appointments are not currently available/
        )
      ) {
        this.result = ScrapeResult.NO;
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
    await this.checkVaccinationStatus(page);
    await page.close();
  }
}
