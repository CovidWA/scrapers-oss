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
import {Page, getPage, sleep, waitAndClick} from '../puppeteer';
import {error, log} from '../logger';
import {processMyChartScreen} from './mychart';
import {Browser} from 'puppeteer-core';

const humanName = 'Seattle Children';
const key = 'seattle_children';
const url = 'https://mychart.seattlechildrens.org/mychart/COVID19#/';

export class SeattleChildrenScraper extends Scraper {
  constructor() {
    super(humanName, key);
  }

  private async tryFillForm(page: Page) {
    try {
      // We already got this page before calling tryFillForm(). The old site Url
      // was not longer useful.

      // await page.goto('https://mychart.seattlechildrens.org/mychart/COVID19#/');
      // Start screening
      await waitAndClick(page, '#InitialTriageCard > div.card-body > div > a');
      // Exposure
      await waitAndClick(page, '#question_Q1 > label:nth-child(4)');
      // Next question
      await page.click(
        '#TriageQuestionCard > div.card-body > div.btn-container.d-flex.justify-content-center > a'
      );
      // Recently diagnosed
      await waitAndClick(page, '#question_Q2 > label:nth-child(4)');
      // Next question
      await page.click(
        '#TriageQuestionCard > div.card-body > div.btn-container.d-flex.justify-content-center > a'
      );
      // Any other vaccine
      await waitAndClick(page, '#question_Q3 > label:nth-child(4)');
      // Next question
      await page.click(
        '#TriageQuestionCard > div.card-body > div.btn-container.d-flex.justify-content-center > a'
      );
      // Antibody therapy
      await waitAndClick(page, '#question_Q4 > label:nth-child(4)');
      // Next question
      await page.click(
        '#TriageQuestionCard > div.card-body > div.btn-container.d-flex.justify-content-center > a'
      );
      // combined other issues/questions - must answer YES
      await waitAndClick(page, '#question_Q5 > label:nth-child(2)');
      // Next question
      await page.click(
        '#TriageQuestionCard > div.card-body > div.btn-container.d-flex.justify-content-center > a'
      );
      // Click "Seattle Childrens"
      await waitAndClick(
        page,
        '#ResultsCard > div.card-body.collapse-padding-t.collapse-padding-sm > div.locationPicker > div'
      );
      // Loading screen
      for (let second = 0; second < 10; ++second) {
        // iframe
        const frame = page
          .frames()
          .find(frame => frame.name() === 'openSchedulingFrame');
        if (frame) {
          break;
        }
        log('Waiting for scheduling frame to load...');
        await sleep(1000);
      }

      this.output = await page.content();
      // iframe
      const frame = page
        .frames()
        .find(frame => frame.name() === 'openSchedulingFrame');
      if (!frame) {
        this.result = ScrapeResult.UNKNOWN;
        this.alarm = true;
        return;
      }
      const myChartResult = await processMyChartScreen(humanName, frame);
      this.result = myChartResult;
      this.output = await frame.content();
      this.alarm = myChartResult === ScrapeResult.UNKNOWN;
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

    await this.tryFillForm(page);

    await page.close();
  }
}
