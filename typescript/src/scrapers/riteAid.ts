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

import {Scraper, ScrapeResult, VaccineType} from '../scraper';
import {
  Clinic,
  getClinicsByUrlKeyword,
  getClinicStatus,
} from '../helpers/getClinics';
import fetch from 'node-fetch';
import {log, error} from '../logger';
import {Page, sleep, waitAndClick, wait, click} from '../puppeteer';
import {Browser} from 'puppeteer-core';

// Generic RiteAid Hybrid Scraper
class RiteAidScraper extends Scraper {
  static initialized = false;
  static page: Page; //global page to preserve eligibility
  static cooldownPeriod = 0;
  static cooldownExpiry = 0; //global throttle cooldown
  static storeSlots: Map<number, number>; //number of appt slots for each store, needs to be global since we're using a global response hook
  static captchaErrorCount = 0;

  recordId: string;
  storeNumber: number;
  address: string;

  constructor(
    humanName: string,
    key: string,
    recordId: string,
    storeNumber: number,
    address: string
  ) {
    super(humanName, key);
    this.recordId = recordId;
    this.storeNumber = storeNumber;
    this.address = address;
  }

  async initGlobal(browser: Browser) {
    //init shared state across all scraper instances
    if (RiteAidScraper.initialized) {
      return;
    }

    RiteAidScraper.initialized = true;

    await getPublicIP();

    if (RiteAidScraper.cooldownExpiry > Date.now()) {
      const cooldownExpiryStr = new Date(
        RiteAidScraper.cooldownExpiry
      ).toLocaleTimeString();
      error(`Scraper in cooldown until ${cooldownExpiryStr}`);
      return;
    }

    RiteAidScraper.storeSlots = new Map<number, number>();

    //open tab for browser scraping and init request interception
    RiteAidScraper.page = await browser.newPage();

    RiteAidScraper.page.on('dialog', async dialog => {
      await dialog.accept();
    });

    RiteAidScraper.page.on('response', async response => {
      if (response.url().includes('ragetavailableappointmentslots')) {
        const slotsResp = await response.json();
        const storeNumberPattern = /storeNumber=[0-9]{4,}/;
        const storeNumberMatch = storeNumberPattern.exec(response.url());
        let storeNumber = -1;
        if (
          storeNumberMatch &&
          storeNumberMatch.length > 0 &&
          storeNumberMatch[0].length > 12
        ) {
          storeNumber = +storeNumberMatch[0].substring(12);
          log(
            `ragetavailableappointmentslots: called for store number ${storeNumber}`
          );
        } else {
          error(
            `ragetavailableappointmentslots could not find store number in url: ${response.url()}`
          );
          return;
        }

        if (slotsResp.Status === 'ERROR') {
          RiteAidScraper.captchaErrorCount++;
          RiteAidScraper.storeSlots.set(storeNumber, -1);
          error(`ragetavailableappointmentslots error: ${slotsResp.ErrMsg}`);
        } else if (slotsResp.Status === 'SUCCESS') {
          try {
            RiteAidScraper.captchaErrorCount = 0;
            let slotCount = 0;
            for (const slot in slotsResp.Data.slots) {
              if (slot !== '10' && slot !== '12') {
                slotCount += slotsResp.Data.slots[slot].length;
              }
            }

            RiteAidScraper.storeSlots.set(storeNumber, slotCount);
            log(
              `ragetavailableappointmentslots success: ${storeNumber} -> ${slotCount}`
            );
          } catch (e) {
            RiteAidScraper.storeSlots.set(storeNumber, -1);
            error(e);
          }
        } else {
          RiteAidScraper.storeSlots.set(storeNumber, -1);
          error(
            `ragetavailableappointmentslots unknown response: ${slotsResp}`
          );
        }
      }
    });

    const puppeteerInternalClient = RiteAidScraper.page['_client'];

    if (puppeteerInternalClient) {
      //block unneeded requests via chrome devtools
      const filters = [
        '*.png',
        '*.svg',
        '*.jpg',
        '*.gif',
        '*.woff',
        '*.woff2',
        'data:image',
        '*adservice*',
        '*analytics*',
        '*doubleclick*',
        'fonts.gstatic.com',
        'virtualearth.net',
        '.facebook.',
        'adobedtm.com',
        'bing.com/fd',
        'bing.com/maps/instrumentation',
        'snapchat.com',
        'twitter.com',
        'pinterest.com',
        'fontawesome.com',
        'rfksrv.com',
        'googletagmanager.com',
      ];

      await puppeteerInternalClient.send('Network.setBlockedURLs', {
        urls: filters,
      });
    }
  }

  async scrape(browser: Browser) {
    await this.initGlobal(browser);

    //cooldown from riteaid's ip blocking
    if (RiteAidScraper.cooldownExpiry > Date.now()) {
      this.skipBackendReporting = true;
      this.result = ScrapeResult.UNKNOWN;
      return;
    }

    //check if store was scraped recently
    try {
      const status = await getClinicStatus(this.recordId);
      const staleness = Date.now() - status.lastChecked * 1000;

      if (
        staleness < 4 * 60 * 1000 &&
        status.Availability !== ScrapeResult.UNKNOWN
      ) {
        //skip if data is less than 4 minutes old
        log(
          `RiteAid #${this.storeNumber} scraped ${
            staleness / 1000
          }s ago, skipping`
        );
        this.skipBackendReporting = true;
        this.result = ScrapeResult.UNKNOWN;
        return;
      }
    } catch (e) {
      //backend error, this should be rare
      error(e);
    }

    try {
      await this.doScrape();

      //reset cooldown
      RiteAidScraper.cooldownPeriod = 0;
      RiteAidScraper.cooldownExpiry = 0;
    } catch (e) {
      error(e.toString());
      this.skipBackendReporting = true;
      this.result = ScrapeResult.UNKNOWN;

      if (!(e instanceof RiteAidBlockedError)) {
        throw e;
      }

      if (RiteAidScraper.cooldownPeriod <= 0) {
        RiteAidScraper.cooldownPeriod = 10 * 1000; //initial period: 10s (skip to next invocation)
      } else if (RiteAidScraper.cooldownPeriod <= 10000) {
        RiteAidScraper.cooldownPeriod = 6 * 60 * 1000; // then 6 minutes (skip 2 invocations)
      } else if (RiteAidScraper.cooldownPeriod <= 24 * 60 * 1000) {
        RiteAidScraper.cooldownPeriod *= 2; //then doubling every time
      } else {
        //max cooldown of 48 minutes
        RiteAidScraper.cooldownPeriod = 48 * 60 * 1000;
      }

      RiteAidScraper.cooldownExpiry =
        Date.now() + RiteAidScraper.cooldownPeriod;
      error(
        `Throttling detected, disabling scrapers for ${RiteAidScraper.cooldownPeriod}ms...`
      );
    }
  }

  async doScrape() {
    // Perform scraping

    // first scrape with api
    const apiResult = await this.checkWithAPI();

    if (apiResult === ScrapeResult.YES) {
      //if api returns yes, confirm with browser

      if (RiteAidScraper.captchaErrorCount > 9) {
        log(
          `RiteAid #${this.storeNumber} too many recaptcha errors, skipping browser scrape`
        );
        this.skipBackendReporting = true;
        this.result = ScrapeResult.UNKNOWN;
        return;
      } else if (
        RiteAidScraper.captchaErrorCount > 0 &&
        RiteAidScraper.captchaErrorCount % 2 === 0
      ) {
        log(
          `RiteAid #${this.storeNumber} recaptcha block detected, resetting...`
        );

        const browser: Browser = RiteAidScraper.page.browser();
        await RiteAidScraper.page.close();
        RiteAidScraper.initialized = false;
        this.initGlobal(browser);
        await sleep(5000);
      }

      //try 3 times
      const maxRetries = 2;
      for (let retries = 0; ; retries++) {
        try {
          this.result = await this.checkWithBrowser(RiteAidScraper.page);
        } catch (e) {
          if (e.constructor.name === 'TimeoutError') {
            if (retries < maxRetries) {
              log(`RiteAid #${this.storeNumber} ${e}`);
              log(`RiteAid #${this.storeNumber} retrying...`);
              continue;
            } else {
              error(
                `RiteAid #${this.storeNumber} too many timeouts, max retries exceeded`
              );
            }
          }
          throw new RiteAidBlockedError(e.toString());
        }
        return;
      }
    } else {
      this.result = apiResult;
    }
  }

  async checkWithAPI() {
    const checkSlotsUrl = `https://www.riteaid.com/services/ext/v2/vaccine/checkSlots?storeNumber=${this.storeNumber}`;

    // Uses undocumented RiteAid API
    let response = await fetchWithRetries(checkSlotsUrl, {
      method: 'get',
      headers: {Accept: 'application/json'},
      timeout: 3000,
    });

    // Logging for Lambda
    console.log(
      `RiteAid #${this.storeNumber} response: ` +
        (await response.clone().text())
    );

    if (!response.ok) {
      return ScrapeResult.UNKNOWN;
    }

    let respJson = await response.json();
    // If the response has an error, we don't know anything about availability
    if (respJson['Status'] !== 'SUCCESS') {
      return ScrapeResult.UNKNOWN;
    }

    const vaccineSlots = respJson['Data']['slots'];
    let hasSlot = false;
    for (const slot in vaccineSlots) {
      if (slot !== '10' && slot !== '12' && vaccineSlots[slot]) {
        if (!this.types) {
          this.types = [];
        }

        if (slot === '9' && !this.types.includes(VaccineType.MODERNA)) {
          this.types.push(VaccineType.MODERNA);
        } else if (slot === '11' && !this.types.includes(VaccineType.PFIZER)) {
          this.types.push(VaccineType.PFIZER);
        } else if (slot === '13' && !this.types.includes(VaccineType.JOHNSON)) {
          this.types.push(VaccineType.JOHNSON);
        }

        hasSlot = true;
      }
    }

    log(`RiteAid #${this.storeNumber} vaccine types: ${this.types}`);

    if (!hasSlot) {
      return ScrapeResult.NO;
    }

    //if there are appt slots, double check that the store offers vaccine at all
    const getStoresUrl = `https://www.riteaid.com/services/ext/v2/stores/getStores?storeNumbers=${this.storeNumber}&attrFilter=PREF-112&fetchMechanismVersion=2`;
    response = await fetchWithRetries(getStoresUrl, {
      method: 'get',
      headers: {Accept: 'application/json'},
      timeout: 3000,
    });

    if (!response.ok) {
      return ScrapeResult.UNKNOWN;
    }

    respJson = await response.json();
    if (respJson['Status'] !== 'SUCCESS') {
      return ScrapeResult.UNKNOWN;
    } else if (respJson['Data']['stores'].length > 0) {
      console.log(`RiteAid #${this.storeNumber} api reports availability`);
      return ScrapeResult.YES;
    } else {
      console.log(
        `RiteAid #${this.storeNumber} has slots but does NOT offer covid vaccine`
      );
      return ScrapeResult.NO;
    }
  }

  async checkWithBrowser(page: Page) {
    console.log(`RiteAid #${this.storeNumber} loading page...`);

    await page.goto('https://www.riteaid.com/pharmacy/apt-scheduler');

    await wait(page, 'button[id=btn-find-store]', 15000);

    if (await wait(page, 'div.error-modal', 500, true, true)) {
      await this.fillEligibility(page);

      if (await wait(page, 'div.release_appt', 5000, true, true)) {
        await waitAndClick(page, 'div.release_appt', 1000, 0, 1);
      }
    }

    if (await wait(page, 'div.release_appt', 500, true, true)) {
      await waitAndClick(page, 'div.release_appt', 1000, 0, 1);
    }

    console.log(
      `RiteAid #${this.storeNumber} searching for store on website...`
    );

    await page.$eval('#covid-store-search', (el: Element) => {
      const input = el as HTMLInputElement;
      if (input) {
        input.value = '';
      }
    });
    await page.type('input[name=covid-store-search]', this.address);
    await waitAndClick(page, 'button[id=btn-find-store]', 5000, 0, 10);

    const storeSel = `a.covid-store__store__anchor--unselected[data-loc-id='${this.storeNumber}']`;
    if (!(await wait(page, storeSel, 5000, true, true))) {
      error(`Rite-Aid: store number ${this.storeNumber} not found in search!`);
      this.skipBackendReporting = true;
      return ScrapeResult.UNKNOWN;
    }

    console.log(`RiteAid #${this.storeNumber} clicking on store...`);
    await click(page, storeSel, 5);

    await waitAndClick(
      page,
      'button:not([disabled])[id=continue]',
      1000,
      0,
      10,
      500
    );

    for (let i = 0; i < 30; i++) {
      await sleep(500);
      const numAppts = RiteAidScraper.storeSlots.get(this.storeNumber);
      if (numAppts !== undefined) {
        if (numAppts < 0) {
          //error encountered
          this.skipBackendReporting = true;
          return ScrapeResult.UNKNOWN;
        } else if (numAppts > this.limitedThreshold) {
          return ScrapeResult.YES;
        } else if (numAppts > 0) {
          return ScrapeResult.LIMITED;
        } else {
          return ScrapeResult.NO;
        }
      }
    }

    error(
      `Rite-Aid: store number ${this.storeNumber} did not receive ragetavailableappointmentslots response`
    );
    this.skipBackendReporting = true;
    return ScrapeResult.UNKNOWN;
  }

  async fillEligibility(page: Page) {
    console.log(
      `RiteAid #${this.storeNumber} filling out eligibility questions...`
    );

    //fill out eligibility screener
    await page.goto('https://www.riteaid.com/covid-vaccine-apt');

    await wait(page, 'button[id=continue]', 15000);

    await page.type('input[name=dateOfBirth]', '01/01/1955');
    await page.type('input[name=zip]', '98101');

    await waitAndClick(
      page,
      'button:not([disabled])[id=continue]',
      5000,
      0,
      10
    );

    await waitAndClick(page, 'a[id=learnmorebttn]', 5000, 1000, 10);

    await wait(page, 'button[id=btn-find-store]', 15000);
  }
}

async function fetchWithRetries(url: string, options = {}) {
  const sleepDurations = [0, 1, 1];

  for (const sleepDuration of sleepDurations) {
    try {
      await sleep(sleepDuration);
      const resp = await fetch(url, options);
      if (resp.ok) {
        return resp;
      } else if (resp.status !== 403 && resp.status !== 429) {
        return resp;
      }
    } catch (e) {
      error(`${e}`);
      if (e.toString().toLowerCase().includes('timeout')) {
        continue;
      }
      if (e.toString().toLowerCase().includes('refused')) {
        continue;
      }

      throw e;
    }
  }

  throw new RiteAidBlockedError(
    'Fetch request refused or timed out repeatedly'
  );
}

class RiteAidBlockedError extends Error {}

export function getRiteAidScrapers(
  clinics: Clinic[],
  segId = 0,
  segments = 0
): RiteAidScraper[] {
  RiteAidScraper.initialized = false;
  RiteAidScraper.captchaErrorCount = 0;

  const riteAidClinics = getClinicsByUrlKeyword('riteaid', clinics);

  const riteAidScrapers = [];
  for (const clinic of riteAidClinics) {
    const storeNumber = parseInt(clinic.key.substring('riteaid_'.length));
    if (
      !isNaN(storeNumber) &&
      (segments === 0 || storeNumber % segments === segId)
    ) {
      log(`Creating scraper for RiteAid #${storeNumber}`);
      riteAidScrapers.push(
        new RiteAidScraper(
          clinic.humanName,
          clinic.key,
          clinic.id,
          storeNumber,
          clinic.address
        )
      );
    }
  }

  return riteAidScrapers;
}

async function getPublicIP(): Promise<string> {
  const ipApiUrls = ['http://ipv4.icanhazip.com', 'http://api.ipify.org/'];

  let lastErr: Error | undefined = undefined;
  for (let retry = 0; retry <= 2; retry++) {
    for (const url of ipApiUrls) {
      const resp = await fetch(url, {
        timeout: 2000,
      }).catch(err => (lastErr = err));
      if (resp.ok) {
        const ip = await resp.text();
        log(`Public IPv4 Address: ${ip}`);
        return ip;
      }
    }
  }

  throw lastErr;
}
