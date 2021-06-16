import fetch from 'node-fetch';

import {Browser} from 'puppeteer-core';
import {Scraper, ScrapeResult, RequestData} from '../scraper';
import * as config from '../../config.json';
import {error} from '../logger';
import {
  ApiRequestCredentials,
  CredentialManager,
  ReCAPTCHAServiceError,
} from '../helpers/albertsonsCredentialManager';

export interface Store {
  humanName: string;
  key: string;
  externalClientId: string;
  storeId: string;
  url: string;
  isRegisteredCompany: boolean;
}

interface LocationResponse {
  name: string;
  timezone: string;
  clientName: string;
}

interface DaySlotRequest {
  slotsYear: string;
  slotsMonth: string;
  companyName: string;
  location: string;
  locationTimezone: string;
}

interface DaySlotResponse {
  slotDates: string[];
}

interface TimeSlotRequest {
  eventDate: string;
  companyName: string;
}
interface TimeSlotResponse {
  date: string;
}

type StoreAppointmentResponse = {
  key: string;
  status: ScrapeResult;
};

export class AlbertsonsHybridScraper extends Scraper {
  stores: Store[];
  storeAppointmentResponses: StoreAppointmentResponse[] = [];
  credentialManager: CredentialManager;

  constructor(stores: Store[], credentialManager: CredentialManager) {
    super('Albertsons Hybrid Scraper', '');
    this.stores = stores;
    // this.skipBackendReporting = true;
    this.credentialManager = credentialManager;
  }

  async scrape(browser: Browser) {
    try {
      for (const store of this.stores) {
        const scrapeResult = await this.scrapeStore(store, browser);

        const fullResultData = {key: store.key, status: scrapeResult};

        this.storeAppointmentResponses.push(fullResultData);
      }
    } catch (err) {
      error(
        `2captcha error encountered, skipping remaining stores and raising alarm: ${err}`
      );
      this.alarm = true;
      for (
        let i = this.storeAppointmentResponses.length;
        i < this.stores.length;
        i++
      ) {
        this.storeAppointmentResponses.push({
          key: this.stores[i].key,
          status: ScrapeResult.POSSIBLE,
        });
      }
    }
  }

  protected async reportResult() {
    const responses = await Promise.all(
      this.storeAppointmentResponses.map(async apptResponse => {
        const requestData: RequestData = {
          ...apptResponse,
          secret: config.secret,
        };
        if (
          apptResponse.status === ScrapeResult.POSSIBLE ||
          this.alarm === true
        ) {
          requestData.alarm = true;
        }
        const apiResponse = await this.sendScraperResult(requestData);
        return `${apptResponse.key}=${apiResponse}`;
      })
    );

    return `hybrid scraper reporting multiple statuses: ${responses.join(',')}`;
  }

  private async scrapeStore(
    store: Store,
    browser: Browser
  ): Promise<ScrapeResult> {
    try {
      const [location] = await this.credentialManager.useCredentials(
        browser,
        apiRequestCredentials =>
          this.fetchLocationData(store, apiRequestCredentials),
        {doesRequestDegradeCredentials: false}
      );

      if (!location) {
        return ScrapeResult.NO;
      }

      return this.freeForAllStrategy(location, browser);
    } catch (err) {
      if (err instanceof ReCAPTCHAServiceError) {
        throw err;
      }
      error(`Error scraping ${this.humanName}: ${JSON.stringify(err)}`);
    }

    return ScrapeResult.POSSIBLE;
  }

  // As of 4/29/21, Albertson's appears to have limited their use of recaptcha, meaning refreshing credentials is trivially fast and only as expensive as the compute time in a lambda. Go nuts and figure out granular availability
  private async freeForAllStrategy(
    {clientName, name, timezone}: LocationResponse,
    browser: Browser
  ) {
    const today = new Date();
    const temp = new Date();
    const nextMonth = new Date(temp.setMonth(temp.getMonth() + 1));

    const {
      slotDates: datesThisMonth,
    } = await this.credentialManager.useCredentials(
      browser,
      apiRequestCredentials =>
        this.fetchAppointmentDays(
          {
            slotsMonth: `${today.getMonth() + 1}`,
            slotsYear: `${today.getFullYear()}`,
            companyName: clientName,
            location: name,
            locationTimezone: timezone,
          },
          apiRequestCredentials
        )
    );
    const {
      slotDates: datesNextMonth,
    } = await this.credentialManager.useCredentials(
      browser,
      apiRequestCredentials =>
        this.fetchAppointmentDays(
          {
            slotsMonth: `${nextMonth.getMonth() + 1}`,
            slotsYear: `${nextMonth.getFullYear()}`,
            companyName: clientName,
            location: name,
            locationTimezone: timezone,
          },
          apiRequestCredentials
        )
    );

    const datesToCheck = [...datesThisMonth, ...datesNextMonth];
    let availabileAppointmentCount = 0;
    for (const date of datesToCheck) {
      if (availabileAppointmentCount >= 5) {
        return ScrapeResult.YES;
      }

      const appointmentTimes = await this.credentialManager.useCredentials(
        browser,
        apiRequestCredentials =>
          this.fetchAppointmentTimes(
            {companyName: clientName, eventDate: date},
            apiRequestCredentials
          )
      );
      availabileAppointmentCount += appointmentTimes.length;
    }

    return availabileAppointmentCount === 0
      ? ScrapeResult.NO
      : ScrapeResult.LIMITED;
  }

  // Before 4/29/21 and potentially some time in the future, Albertson's made use of recaptcha, and we may want this option to limit costs should the need arise. Saves $$ at the expense of availability granularity
  private async conservativeStrategy(
    {clientName, name, timezone}: LocationResponse,
    browser: Browser
  ) {
    const currentMonthHasAvailability = await this.doesCurrentMonthHaveAvailability(
      clientName,
      name,
      timezone,
      browser
    );
    if (currentMonthHasAvailability) {
      return ScrapeResult.YES;
    }

    const nextMonthHasAvailability = await this.doesNextMonthHaveAvailability(
      clientName,
      name,
      timezone,
      browser
    );

    return nextMonthHasAvailability ? ScrapeResult.YES : ScrapeResult.NO;
  }

  private async doesCurrentMonthHaveAvailability(
    companyName: string,
    location: string,
    locationTimezone: string,
    browser: Browser
  ): Promise<boolean> {
    const today = new Date();

    const {slotDates} = await this.credentialManager.useCredentials(
      browser,
      apiRequestCredentials =>
        this.fetchAppointmentDays(
          {
            slotsMonth: `${today.getMonth() + 1}`,
            slotsYear: `${today.getFullYear()}`,
            companyName,
            location,
            locationTimezone,
          },
          apiRequestCredentials
        )
    );

    // Current month seems to more reliably return false positives,
    // so we want to verify that there's a day in this month with actual timeslots
    const [furthestAppointmentDate] = slotDates.slice(-1);
    if (!furthestAppointmentDate) {
      return false;
    }

    const appointmentTimes = await this.credentialManager.useCredentials(
      browser,
      apiRequestCredentials =>
        this.fetchAppointmentTimes(
          {companyName, eventDate: furthestAppointmentDate},
          apiRequestCredentials
        )
    );

    return appointmentTimes.length > 0;
  }

  private async doesNextMonthHaveAvailability(
    companyName: string,
    location: string,
    locationTimezone: string,
    browser: Browser
  ): Promise<boolean> {
    const temp = new Date();
    const nextMonth = new Date(temp.setMonth(temp.getMonth() + 1));

    const {slotDates} = await this.credentialManager.useCredentials(
      browser,
      apiRequestCredentials =>
        this.fetchAppointmentDays(
          {
            slotsMonth: `${nextMonth.getMonth() + 1}`,
            slotsYear: `${nextMonth.getFullYear()}`,
            companyName,
            location,
            locationTimezone,
          },
          apiRequestCredentials
        )
    );

    return slotDates.length > 0;
  }

  private async fetchLocationData(
    {externalClientId, storeId, isRegisteredCompany}: Store,
    {csrfKey, cookies}: ApiRequestCredentials
  ): Promise<LocationResponse[]> {
    const loadLocationRequest = new URL(
      'https://kordinator.mhealthcoach.net/loadLocationsForClientAndApptType.do'
    );
    const query: {[key: string]: string} = isRegisteredCompany
      ? {
          accessKey: externalClientId,
          externalClientId: storeId,
          clientIds: storeId,
          instore: 'no',
        }
      : {
          externalClientId,
          clientIds: storeId,
          instore: 'yes',
        };
    loadLocationRequest.search = new URLSearchParams({
      ...query,
      _r: `${Math.floor(Math.random() * 1000000000000000000)}`,
      apptKey: 'COVID_VACCINE_DOSE1_APPT',
      csrfKey,
    }).toString();

    const response = await fetch(loadLocationRequest, {
      headers: {
        cookie: cookies.map(({name, value}) => `${name}=${value}`).join(';'),
        'user-agent': 'covidwa/scrapers',
        accept: '*/*',
        connection: 'keep-alive',
      },
    });

    return response.json();
  }

  private async fetchAppointmentDays(
    slotRequest: DaySlotRequest,
    {csrfKey, cookies}: ApiRequestCredentials
  ): Promise<DaySlotResponse> {
    const loadLocationRequest = new URL(
      'https://kordinator.mhealthcoach.net/loadEventSlotDaysForCoach.do'
    );
    loadLocationRequest.search = new URLSearchParams({
      _r: `${Math.floor(Math.random() * 1000000000000000000)}`,
      csrfKey,
      cva: 'true',
      type: 'registration',
    }).toString();

    const response = await fetch(loadLocationRequest, {
      headers: {
        cookie: cookies.map(({name, value}) => `${name}=${value}`).join(';'),
        'user-agent': 'covidwa/scrapers',
        accept: '*/*',
        connection: 'keep-alive',
        'content-type': 'application/x-www-form-urlencoded',
      },
      method: 'POST',
      body: new URLSearchParams({
        ...slotRequest,
        csrfKey,
        eventType: 'COVID Vaccine Dose 1 Appt',
      }).toString(),
    });

    return response.json();
  }

  private async fetchAppointmentTimes(
    slotRequest: TimeSlotRequest,
    {csrfKey, cookies}: ApiRequestCredentials
  ): Promise<TimeSlotResponse[]> {
    const loadLocationRequest = new URL(
      'https://kordinator.mhealthcoach.net/loadEventSlotsForCoach.do'
    );
    loadLocationRequest.search = new URLSearchParams({
      _r: `${Math.floor(Math.random() * 1000000000000000000)}`,
      csrfKey,
      cva: 'true',
      type: 'registration',
    }).toString();

    const response = await fetch(loadLocationRequest, {
      headers: {
        cookie: cookies.map(({name, value}) => `${name}=${value}`).join(';'),
        'user-agent': 'covidwa/scrapers',
        accept: '*/*',
        connection: 'keep-alive',
        'content-type': 'application/x-www-form-urlencoded',
      },
      method: 'POST',
      body: new URLSearchParams({
        ...slotRequest,
        eventType: 'COVID Vaccine Dose 1 Appt',
      }).toString(),
    });

    return response.json();
  }
}
