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

// This scraper relies on the presence of the following in airtable database's
// scraper_config cell: matchStr = unique string to match against; pf1 = list
// of Pfizer Dose 1 reasonIds, pf2 for 2nd dose, mo1 = list of Moderna Dose 1
// reasonIds, mo2 for 2nd dose, and jnj = list of Janssen (Johnson & Johnson)
// reasonIds.

import {Scraper, ScrapeResult, VaccineType} from '../scraper';
import {Clinic, getClinicsByUrlKeyword} from '../helpers/getClinics';
import fetch from 'node-fetch';
import {error} from '../logger';
import {getPage, Page, sleep} from '../puppeteer';
import {Browser} from 'puppeteer-core';

// Generic TimeTap Url Scraper
class TimeTapScraper extends Scraper {
  url: string;
  scraper_config: string;
  constructor(
    humanName: string,
    key: string,
    url: string,
    scraper_config: string
  ) {
    super(humanName, key);
    this.url = url;
    this.scraper_config = scraper_config;
  }

  async scrape(browser: Browser) {
    // Perform scraping
    const page = await getPage(browser, this.key, this.url);

    await this.checkVaccinationStatus(page);

    await page.close();
  }

  async checkVaccinationStatus(page: Page) {
    // Create Set
    const vaccineTypes = new Set<string>();

    try {
      console.log(this.url + ' for ' + this.key);

      // Make sure scraper_config is populated. We need it later.
      if (!this.scraper_config) {
        this.alarm = true;
        this.result = ScrapeResult.UNKNOWN;
        console.log('Missing scraper_config');
        error(`Error scraping ${this.humanName}: missing scraper_config`);

        return;
      }

      const configObj = JSON.parse(this.scraper_config);
      if (
        !configObj ||
        typeof configObj !== 'object' ||
        !('matchStr' in configObj)
      ) {
        this.alarm = true;
        this.result = ScrapeResult.UNKNOWN;
        console.log('Missing matchStr in scraper_config');
        error(
          `Error scraping ${this.humanName}: missing matchStr in scraper_config`
        );
        return;
      }
      // console.log('scraper_config: ' + this.scraper_config);

      const domain = new URL(this.url).host;
      await sleep(1000);
      this.output = await page.content();

      //Get cst bearer token.
      const token = this.output.match(/cst:[^"]+/);
      const errorMessage1 = this.output.match(
        /Bookings on this website have been temporarily disabled/
      );
      const errorMessage2 = this.output.match(
        /could not load the requested scheduler/
      );

      if (errorMessage1 || errorMessage2) {
        this.result = ScrapeResult.NO;
        return;
      }

      if (!token) {
        // If the response has an error, we can't know anything about availability.
        console.log('Unable to find timetap token on page');
        // Special case for Pacifica: return NO instead of UNKNOWN
        if (this.url.match(/pacifica/gi)) {
          this.result = ScrapeResult.NO;
        } else {
          this.alarm = true;
          this.result = ScrapeResult.UNKNOWN;
          error(
            `Error scraping ${this.humanName}: unable to find timetap token`
          );
        }
        return;
      }

      // Refresh the token.
      const refreshTokenUrl = `https://${domain}/businessWeb/csapi/cs/refreshSession?sessionToken=${token[0]}`;
      const refeshResponse = await fetch(refreshTokenUrl, {
        method: 'get',

        headers: {
          Accept: 'application/json',
        },
      });
      if (!refeshResponse.ok) {
        this.alarm = true;
        this.result = ScrapeResult.UNKNOWN;
        console.log('Unable to refresh token');
        error(`Error scraping ${this.humanName}: unable to refresh token`);

        return;
      }

      // Get location info.
      const respJson = await refeshResponse.json();
      const getLocationInformation = `https://${domain}/businessWeb/csapi/cs/locations`;
      const getLocationResponse = await fetch(getLocationInformation, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token[0]}`,
          Accept: 'application/json',
          'content-type': 'application/json',
          Origin: `https://${domain}`,
          'accept-language': 'en-US',
        },
        body: JSON.stringify({
          debug: false,
          businessId: respJson.businessId,
          schedulerLinkId: respJson.schedulerLinkId,
          clientTimeZone: 'America/Los_Angeles',
          clientTimeZoneId: 66,
          locale: 'en-US',
        }),
      });
      if (!getLocationResponse.ok) {
        this.alarm = true;
        this.result = ScrapeResult.UNKNOWN;
        console.log('Unable to get location information');
        error(
          `Error scraping ${this.humanName}: unable to get location information`
        );

        return;
      }
      const getLocationJson = await getLocationResponse.json();
      if (getLocationJson.length === 0) {
        console.log('Location no longer in use');
        this.result = ScrapeResult.NO;
        return;
      }

      // Use matchStr for matching, otherwise availability for one
      // can look like availability for a different location (because locationId
      // is not unique, it's merely the pharmacy owner's id).
      const matchStr = configObj['matchStr'].toLowerCase();

      // Try matching against every location name.
      let isMatch = false;
      let locName = '';
      let locId = 0;
      for (let i = 0; i < Object.keys(getLocationJson).length; i++) {
        // console.log('Location name: ' + getLocationJson[i].locationName.toLowerCase());
        locName = getLocationJson[i].locationName.toLowerCase();
        isMatch = locName.includes(matchStr);
        if (isMatch) {
          locId = getLocationJson[i].locationId;
          // console.log('Got a location match for id = ' + locId);
          break;
        }
      }

      if (!isMatch) {
        // console.log('Mismatch ' + matchStr + ' vs ' + locName);
        this.result = ScrapeResult.NO;
        return;
      }

      const d = new Date();
      const monthDate = d.getMonth() + 1; // getMonth() returns 0..11

      // Get the reasonIds from scraper_config info in configObj.
      const idPfizer1: number[] = configObj['pf1'];
      const idPfizer2: number[] = configObj['pf2'];
      const idModerna1: number[] = configObj['mo1'];
      const idModerna2: number[] = configObj['mo2'];
      const idJohnson: number[] = configObj['jnj'];

      const reasonIds = [
        ...idPfizer1,
        ...idPfizer2,
        ...idModerna1,
        ...idModerna2,
        ...idJohnson,
      ];

      for (const reason of reasonIds) {
        console.log('reasonId:' + reason);
        const getSlotsUrl = `https://${domain}/businessWeb/csapi/cs/availability/class/month/2021/${monthDate}?findBestMonth=true&waitListMode=false`;
        // Here is an alternate form of API URL:
        // const altGetSlotsUrl = `https://${domain}/businessWeb/csapi/cs/availability/month/2021/${monthDate}?findBestMonth=true&duration=5&waitListMode=false`;

        const getSlotsResponse = await fetch(getSlotsUrl, {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${token[0]}`,
            Accept: 'application/json',
            'content-type': 'application/json',
          },
          body: JSON.stringify({
            debug: false,
            businessId: respJson.businessId,
            locationIdList: [locId],
            reasonIdList: [reason],
            schedulerLinkId: respJson.schedulerLinkId,
            clientTimeZone: 'America/Los_Angeles',
            clientTimeZoneId: 66,
            locale: 'en-US',
          }),
        });

        if (!getSlotsResponse.ok) {
          this.alarm = true;
          this.result = ScrapeResult.UNKNOWN;
          console.log(
            `Unable to get slot response: ${getSlotsResponse.statusText}`
          );
          error(
            `Error scraping ${this.humanName}: unable to get slot response`
          );

          return;
        }
        const slotUrlJson = await getSlotsResponse.json();

        if (slotUrlJson['openDays'].length > 0) {
          // console.log('openDays: ' + slotUrlJson['openDays'].length);
          this.result = ScrapeResult.YES;
          if (idPfizer1.includes(reason)) {
            console.log('Pfizer 1st dose available');
            vaccineTypes.add(VaccineType.PFIZER);
          } else if (idPfizer2.includes(reason)) {
            console.log('Pfizer 2nd dose available');
            vaccineTypes.add(VaccineType.PFIZER);
          } else if (idModerna1.includes(reason)) {
            console.log('Moderna 1st dose available');
            vaccineTypes.add(VaccineType.MODERNA);
          } else if (idModerna2.includes(reason)) {
            console.log('Moderna 2nd dose available');
            vaccineTypes.add(VaccineType.MODERNA);
          } else if (idJohnson.includes(reason)) {
            console.log('Johnson & Johnson single dose available');
            vaccineTypes.add(VaccineType.JOHNSON);
          }
        } else {
          // If a previous loop produced a YES, don't wipe it out with NO
          if (this.result !== ScrapeResult.YES) {
            this.result = ScrapeResult.NO;
          }
        }
      }
    } catch (err) {
      this.result = ScrapeResult.UNKNOWN;
      this.alarm = true;
      error(`Error scraping ${this.humanName}: ${JSON.stringify(err)}`);
    }
    // Cast the Set to the result.types array.
    this.types = Array.from(vaccineTypes);
  }
}

export function getTimeTapScrapers(clinics: Clinic[]): TimeTapScraper[] {
  const timeTapClinics = getClinicsByUrlKeyword('timetap', clinics);

  const timeTapScrapers = [];
  for (const clinic of timeTapClinics) {
    if (clinic.key) {
      if (!clinic.key.match(/Xtimetap/)) {
        timeTapScrapers.push(
          new TimeTapScraper(
            clinic.humanName,
            clinic.key,
            clinic.url,
            clinic.scraper_config
          )
        );
      } else {
        console.log('skipping key: ' + clinic.key);
      }
    }
  }
  return timeTapScrapers;
}
