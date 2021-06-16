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

import {log, error} from '../logger';
import {Page, Frame, sleep, isVisible} from '../puppeteer';
import {ScrapeResult} from '../scraper';

export async function processMyChartScreen(
  humanName: string,
  page: Page | Frame
) {
  try {
    // Loading screen
    const errorText = /Your request could not be carried out because of an error/;
    const loadingSelector = 'div.loadingmessage';
    const firstApptSelector = 'a.firstslot';
    const noApptSelector = 'div.errormessage';
    for (let second = 0; second < 30; ++second) {
      await sleep(1000);
      const content = await page.content();
      const hasError = content.match(errorText);
      if (hasError) {
        log('Error message found, returning UNKNOWN.');
        return ScrapeResult.UNKNOWN;
      }
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
