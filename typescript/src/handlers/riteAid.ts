import {getClinics} from '../helpers/getClinics';
import {getRiteAidScrapers} from '../scrapers/riteAid';
import {ScraperHandler} from '../executor';
import {Scraper} from '../scraper';

const riteAidHandlerGenerator = (segId = 0, segments = 0) => async (
  event: undefined,
  context: undefined,
  callback: Function
) => {
  const clinics = await getClinics();
  const riteAidScrapers = getRiteAidScrapers(clinics, segId, segments);
  const scrapers: Scraper[] = [...riteAidScrapers];

  const scraperHandler = ScraperHandler(...scrapers);
  await scraperHandler(event, context, callback);
};

//quad redundant scrapers
export const riteAidHandler1 = riteAidHandlerGenerator();
export const riteAidHandler2 = riteAidHandlerGenerator();
export const riteAidHandler3 = riteAidHandlerGenerator();
export const riteAidHandler4 = riteAidHandlerGenerator();
export const riteAidHandler5 = riteAidHandlerGenerator();
export const riteAidHandler6 = riteAidHandlerGenerator();
export const riteAidHandler7 = riteAidHandlerGenerator();
export const riteAidHandler8 = riteAidHandlerGenerator();
