import fetch from 'node-fetch';

import {getClinics, getClinicsByUrlKeyword} from '../helpers/getClinics';
import {ScraperHandler} from '../executor';
import {AlbertsonsHybridScraper, Store} from '../scrapers/albertsonsHybrid';
import {log} from '../logger';
import {AlbertsonsCredentialManager} from '../helpers/albertsonsCredentialManager';
import {
  CachedCredentials,
  retrieveCachedCredentials,
  sendCredentialsToCache,
} from '../helpers/cachedCredentials';
import {
  AlbertsonsFlatFileScraper,
  FlatFileResponse,
} from '../scrapers/albertsonsFlatFile';

const PORTLAND_SEARCH_CLIENT_ID = '1610137270791';
const SEATTLE_SEARCH_CLIENT_ID = '1610137564207';

const seattleSearchClientHandlerGenerator = (
  segmentNumber: number,
  segmentCount: number
) => async (event: undefined, context: undefined, callback: Function) => {
  const [validStores, allVaccineAvailability] = await Promise.all([
    loadStoreData().then(stores =>
      stores.filter(
        ({externalClientId}) => externalClientId === SEATTLE_SEARCH_CLIENT_ID
      )
    ),
    fetch(
      'https://s3-us-west-2.amazonaws.com/mhc.cdn.content/vaccineAvailability.json'
    ).then(async response => (await response.json()) as FlatFileResponse[]),
  ]);

  const segmentLength = Math.ceil(validStores.length / segmentCount);
  const start = (segmentNumber - 1) * segmentLength;
  const end = segmentNumber * segmentLength;
  const storeShare = validStores.slice(start, end);

  const storeIds = new Set(storeShare.map(({storeId}) => storeId));
  const initialStoreStatusLookup = allVaccineAvailability
    .filter(({id}) => storeIds.has(id))
    .reduce((acc, currentAvailability) => {
      acc[currentAvailability.id] = currentAvailability;
      return acc;
    }, {} as {[key: string]: FlatFileResponse});

  // id corresponds to a store that uses the Seattle Search Client
  const credentialManager = AlbertsonsCredentialManager.forSearchClient(
    '1600115131031'
  );
  log(
    `Scraping segment of stores that use the Seattle Search Client: ${start} to ${end} (${storeShare.length})`
  );
  const scrapers = chunkArray(storeShare, 10).map(
    storesChunk =>
      new AlbertsonsFlatFileScraper(
        storesChunk,
        initialStoreStatusLookup,
        credentialManager
      )
  );

  const scraperHandler = ScraperHandler(...scrapers);
  await scraperHandler(event, context, callback);
};

function chunkArray<T>(items: T[], chunkSize: number): T[][] {
  return [...Array(Math.ceil(items.length / chunkSize))].map((_, i) =>
    items.slice(i * chunkSize, i * chunkSize + chunkSize)
  );
}

// First half of Albertson's stores that require credentials from the Seattle Search Client
export const albertsonsHandler1 = seattleSearchClientHandlerGenerator(1, 2);

// Second half of Albertson's stores that require credentials from the Seattle Search Client
export const albertsonsHandler2 = seattleSearchClientHandlerGenerator(2, 2);

// All stores that require credentials from the Portland Search Client
export const albertsonsHandler3 = async (
  event: undefined,
  context: undefined,
  callback: Function
) => {
  const [validStores, allVaccineAvailability] = await Promise.all([
    loadStoreData().then(stores =>
      stores.filter(
        ({externalClientId}) => externalClientId === PORTLAND_SEARCH_CLIENT_ID
      )
    ),
    fetch(
      'https://s3-us-west-2.amazonaws.com/mhc.cdn.content/vaccineAvailability.json'
    ).then(async response => (await response.json()) as FlatFileResponse[]),
  ]);

  const storeIds = new Set(validStores.map(({storeId}) => storeId));
  const initialStoreStatusLookup = allVaccineAvailability
    .filter(({id}) => storeIds.has(id))
    .reduce((acc, currentAvailability) => {
      acc[currentAvailability.id] = currentAvailability;
      return acc;
    }, {} as {[key: string]: FlatFileResponse});

  // id corresponds to the first store that indicated this Seattle/Portland split: safeway_19_1078
  const credentialManager = AlbertsonsCredentialManager.forSearchClient(
    '1600113329481'
  );
  log(
    `Scraping ${validStores.length} stores that use the Portland Search Client`
  );
  const scrapers = chunkArray(validStores, 10).map(
    storesChunk =>
      new AlbertsonsFlatFileScraper(
        storesChunk,
        initialStoreStatusLookup,
        credentialManager
      )
  );

  const scraperHandler = ScraperHandler(...scrapers);
  await scraperHandler(event, context, callback);
};

// Clinics that fall under a "registered company" designation - relies on cached credentials between runs for minimizing 2captcha waste
export const albertsonsHandler4 = async (
  event: undefined,
  context: undefined,
  callback: Function
) => {
  const [validStores, cachedCredentials] = await Promise.all([
    loadStoreData().then(stores =>
      stores.filter(
        ({externalClientId}) =>
          externalClientId !== PORTLAND_SEARCH_CLIENT_ID &&
          externalClientId !== SEATTLE_SEARCH_CLIENT_ID
      )
    ),
    retrieveCachedCredentials(),
  ]);
  log(
    `Retrieved cached credentials:\n${JSON.stringify(
      cachedCredentials,
      null,
      2
    )}`
  );

  const credentialManagers = validStores.reduce((acc, {key, url}) => {
    const credentialsForStore = cachedCredentials[key];
    acc[key] = AlbertsonsCredentialManager.forRegisteredCompany({
      initialCredentials: credentialsForStore?.credentials,
      initialCredentialCount: credentialsForStore?.useCount,
      overrideUrl: url,
    });
    return acc;
  }, {} as {[key: string]: AlbertsonsCredentialManager});

  const scraperHandler = ScraperHandler(
    ...validStores.map(
      store =>
        new AlbertsonsHybridScraper([store], credentialManagers[store.key])
    )
  );
  await scraperHandler(event, context, callback);

  log('Successfully scraped registered companies, caching credentials...');

  const updatedCredentials: CachedCredentials = validStores.reduce(
    (acc, {key}) => {
      acc[key] = credentialManagers[key].currentCredentialInfo;
      return acc;
    },
    {} as CachedCredentials
  );
  const result = await sendCredentialsToCache(updatedCredentials);
  if (result) {
    log('Successfully cached credentials for the next run');
  } else {
    log(
      'Credentials were not cached for the next run, expect a refresh attempt in a subsequent run'
    );
  }
};

async function loadStoreData(): Promise<Store[]> {
  const clinics = await getClinics();
  const coreClinicData = getClinicsByUrlKeyword('mhealth', clinics);
  return coreClinicData
    .filter(({url}) => new URL(url).searchParams.has('clientId'))
    .map(({humanName, key, url}) => {
      const {pathname, searchParams} = new URL(url);
      const [, , externalClientId] = pathname.split('/');
      const storeId = searchParams.get('clientId')!;

      return {
        humanName,
        key,
        externalClientId,
        storeId,
        url,
        isRegisteredCompany:
          externalClientId !== PORTLAND_SEARCH_CLIENT_ID &&
          externalClientId !== SEATTLE_SEARCH_CLIENT_ID,
      };
    });
}
