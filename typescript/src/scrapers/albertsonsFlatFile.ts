import {Browser} from 'puppeteer-core';

import * as config from '../../config.json';
import {CredentialManager} from '../helpers/albertsonsCredentialManager';
import {log} from '../logger';
import {RequestData, Scraper, ScrapeResult, VaccineType} from '../scraper';
import {AlbertsonsHybridScraper, Store} from './albertsonsHybrid';

type AlbertsonsDrugName = 'Moderna' | 'Pfizer' | 'JnJ';

export type FlatFileResponse = {
  id: string;
  availability: string;
  drugName?: AlbertsonsDrugName[];
};

type StoreAppointmentResponse = {
  key: string;
  status: ScrapeResult;
  scraperTags: VaccineType[];
};

interface StoreWithVaccineAvailability extends Store {
  vaccineTypes: VaccineType[];
}

export class AlbertsonsFlatFileScraper extends Scraper {
  stores: Store[];
  preliminaryVaccineAvailabiltyLookup: {[key: string]: FlatFileResponse};
  credentialManager: CredentialManager;
  storeAppointmentResponses: StoreAppointmentResponse[] = [];

  constructor(
    stores: Store[],
    preliminaryVaccineAvailabiltyLookup: {[key: string]: FlatFileResponse},
    credentialManager: CredentialManager
  ) {
    super('Albertsons Hybrid Scraper', '');
    this.stores = stores;
    this.preliminaryVaccineAvailabiltyLookup = preliminaryVaccineAvailabiltyLookup;
    this.credentialManager = credentialManager;
  }

  async scrape(browser: Browser) {
    const storesWithPossibleAvailability: StoreWithVaccineAvailability[] = [];
    for (const store of this.stores) {
      const {availability, drugName} = this.preliminaryVaccineAvailabiltyLookup[
        store.storeId
      ];
      const vaccineTypes = parseVaccineTypeInfo(drugName ?? []);
      if (availability === 'no') {
        this.storeAppointmentResponses.push({
          key: store.key,
          status: ScrapeResult.NO,
          scraperTags: vaccineTypes,
        });
      } else {
        storesWithPossibleAvailability.push({...store, vaccineTypes});
      }
    }
    if (storesWithPossibleAvailability.length > 0) {
      const hybridScraper = new AlbertsonsHybridScraper(
        storesWithPossibleAvailability,
        this.credentialManager
      );
      await hybridScraper.scrape(browser);
      this.storeAppointmentResponses.push(
        ...hybridScraper.storeAppointmentResponses.map((response, i) => ({
          ...response,
          scraperTags: storesWithPossibleAvailability[i].vaccineTypes,
        }))
      );
    }
  }

  async report() {
    this.storeAppointmentResponses.forEach(res => log(JSON.stringify(res)));
    return super.report();
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
}

function parseVaccineTypeInfo(types: AlbertsonsDrugName[]): VaccineType[] {
  return types.map(type => {
    switch (type) {
      case 'Moderna':
        return VaccineType.MODERNA;
      case 'Pfizer':
        return VaccineType.PFIZER;
      case 'JnJ':
        return VaccineType.JOHNSON;
    }
  });
}
