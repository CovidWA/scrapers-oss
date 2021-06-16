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
import RecaptchaPlugin, {
  PuppeteerExtraPluginRecaptcha,
} from 'puppeteer-extra-plugin-recaptcha';

import {Clinic, getClinicsByUrlKeyword} from '../helpers/getClinics';
import {Browser, HTTPResponse} from 'puppeteer-core';
import {error} from '../logger';
import * as config from '../../config.json';

interface SlotResponse {
  slotDates: string[];
}

// Generic AlbertsonsCompany Url Scraper
class AlbertsonsCompaniesScraper extends Scraper {
  url: string;
  zipCode: string;
  recaptcha: PuppeteerExtraPluginRecaptcha;

  constructor(
    humanName: string,
    key: string,
    zipCode: string,
    recaptcha: PuppeteerExtraPluginRecaptcha
  ) {
    super(humanName, key);

    this.zipCode = zipCode;
    this.url = 'https://www.mhealthappointments.com/covidappt';
    this.recaptcha = recaptcha;
    this.skipBackendReporting = true;
  }

  async scrape(browser: Browser) {
    const page = await getPage(browser, this.key, this.url);

    try {
      await this.completeEntryQuestionnaire(this.zipCode, page, this.recaptcha);

      const hasAnyAppointments = await this.checkVaccinationStatus(page);
      this.result = hasAnyAppointments ? ScrapeResult.YES : ScrapeResult.NO;
    } catch (err) {
      error(`Error scraping ${this.humanName}: ${JSON.stringify(err)}`);
      this.result = ScrapeResult.POSSIBLE;
      this.alarm = true;
    } finally {
      this.output = await page.content();
    }

    await page.close();
  }

  private async completeEntryQuestionnaire(
    zipCode: string,
    page: Page,
    recaptcha: PuppeteerExtraPluginRecaptcha
  ) {
    const zipSearchBox = await page.waitForSelector(
      '#covid_vaccine_search_input',
      {visible: true}
    );
    await zipSearchBox?.type(zipCode);

    await page.click('#fiftyMile-covid_vaccine_search');
    await page.waitForTimeout(1000);

    await page.click('[onclick="covidVaccinationZipSearch()"]');
    const consentButton = await page.waitForSelector('#attestation_1002', {
      visible: true,
    });

    const [, {error}] = await Promise.all([
      consentButton!.click(),
      // This averages 30s and costs $2.99 per 1000 solves
      recaptcha.solveRecaptchas(page),
    ]);
    if (error) {
      throw error;
    }

    await page.click('#covid_vaccine_search_questions_submit .btn-primary');
  }

  private async checkVaccinationStatus(page: Page): Promise<boolean> {
    await page.waitForSelector('#appointmentType-type', {visible: true});
    await page.select('#appointmentType-type', 'object:60');

    // This indicates we've changed sections in the appointment setup questions
    const appointmentTypeButton = await page.waitForXPath(
      '(//*[@id="covid19-reg-v2"]//button[contains(@class,"next-button")])[1][not(contains(@class,"ng-hide"))]',
      {visible: true}
    );
    await appointmentTypeButton!.click();
    const scheduleButton = await page.waitForXPath(
      '(//*[@id="covid19-reg-v2"]//button[contains(@class,"next-button")])[2][not(contains(@class,"ng-hide"))]',
      {visible: true}
    );
    await scheduleButton!.click();

    let hasAnyAppointments = false;

    const storeSelectionDropdownCount = (await page.$$('#item-type option'))
      .length;
    for (let i = storeSelectionDropdownCount - 1; i >= 0; i--) {
      if (
        await this.findAppointmentsForCurrentCalendar(
          page,
          page.select('#item-type', `${i}`)
        )
      ) {
        hasAnyAppointments = true;
        break;
      }
    }

    return hasAnyAppointments;
  }

  private async findAppointmentsForCurrentCalendar(
    page: Page,
    triggerEvent: Promise<unknown>
  ): Promise<boolean> {
    const [initialMonthResponse] = await Promise.all([
      this.waitForCalendarLoad(page),
      triggerEvent,
    ]);
    const initialMonth: SlotResponse = await initialMonthResponse.json();
    if (initialMonth.slotDates.length > 0) {
      return true;
    }

    const [secondMonthResponse] = await Promise.all([
      this.waitForCalendarLoad(page),
      page.click('.uib-daypicker .uib-right'),
    ]);
    const secondMonth: SlotResponse = await secondMonthResponse.json();
    const result = secondMonth.slotDates.length > 0;

    // Clicking the dropdown to change locations does NOT reset the calendar to the current month so we need to force it here
    await page.click('.uib-daypicker .uib-left');

    return result;
  }

  private async waitForCalendarLoad(page: Page) {
    return page.waitForResponse((response?: HTTPResponse) =>
      response
        ?.request()
        ?.url()
        ?.includes('loadEventSlotDaysForCoach.do?cva=true&type=registration')
    );
  }
}

export function getAlbertsonsCompaniesScrapers(
  clinics: Array<Clinic>
): AlbertsonsCompaniesScraper[] {
  // can be mhealthcheckin or mhealthcoach
  const coreClinicData = getClinicsByUrlKeyword('mhealthc', clinics);
  const recaptcha = RecaptchaPlugin({
    provider: {
      id: '2captcha',
      // Needs a (paid :( ) 2captcha account
      token: config.recaptchaBypassServiceToken,
    },
    visualFeedback: true,
  });

  // The url's now automatically redirect to the base address set in AlbertsonsCompaniesScraper class
  return coreClinicData
    .filter(({url}) => new URL(url).searchParams.has('zipCode'))
    .map(({humanName, key, url}) => {
      const zipCode = new URL(url).searchParams.get('zipCode')!;
      return new AlbertsonsCompaniesScraper(humanName, key, zipCode, recaptcha);
    });
}
