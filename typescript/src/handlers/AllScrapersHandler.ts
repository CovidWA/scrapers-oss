import {getClinics} from '../helpers/getClinics';
import {ScraperHandler} from '../executor';
//import {getSolvHealthScrapers} from '../scrapers/solvHealth';
import {Scraper} from '../scraper';
// import {ShuksanFamilyMedicineScraper} from '../scrapers/shuksanFamilyMedicine';
import {SeattleChildrenScraper} from '../scrapers/seattlechildren';
// import {HarborViewScraper} from '../scrapers/harborview';
// import {LakeWashingtonScraper} from '../scrapers/lakewashington';
// import {ProsserMemorialHealthScraper} from '../scrapers/prosserMemorialHealth';
// import {NorthSoundPediatricsScraper} from '../scrapers/northSoundPediatrics';
// import {getClallamHHSScrapers} from '../scrapers/clallamHHS';
// import {EvergreenFairgroundScraper} from '../scrapers/evergreenFairground';
// import {ArlingtonMunicipalAirportScraper} from '../scrapers/arlingtonMunicipalAirport';
// import {CleElumCentennial} from '../scrapers/CleElumCentennial';
import {getCalendlyScrapers} from '../scrapers/calendly';
import {getCHIFranciscanScrapers} from '../scrapers/chif';
// import {getMulticareScrapers} from '../scrapers/multicare';
import {getTimeTapScrapers} from '../scrapers/timeTap';
import {getPeaceHealthScrapers} from '../scrapers/peaceHealth';
// import {MemberPlusFamilyHealthScraper} from '../scrapers/memberPlusFamilyHealth';
import {getMulticareDirectScrapers} from '../scrapers/multicareDirect';
// import {ARCpointLabsScraper} from '../scrapers/arcpointLabs';

export const handler = async (
  event: undefined,
  context: undefined,
  callback: Function
) => {
  const clinics = await getClinics();
  const calendlyScrapers = getCalendlyScrapers(clinics);
  const chiFranciscanScrapers = getCHIFranciscanScrapers(clinics);
  //const clallamHHSScraper = getClallamHHSScrapers(clinics);
  // const multicareScrapers = getMulticareScrapers();
  const timeTapScraper = getTimeTapScrapers(clinics);
  const peaceHealthScrapers = getPeaceHealthScrapers(clinics);
  const multicareDirectScrapers = getMulticareDirectScrapers(clinics);

  const scrapers: Scraper[] = [
    // new ExampleScraper(),
    // new ShuksanFamilyMedicineScraper(),
    new SeattleChildrenScraper(),
    // new HarborViewScraper(),
    // new LakeWashingtonScraper(),
    // new ProsserMemorialHealthScraper(),
    // new NorthSoundPediatricsScraper(),
    // new EvergreenFairgroundScraper(),
    // new ArlingtonMunicipalAirportScraper(),
    // new CleElumCentennial(),
    ...calendlyScrapers,
    ...chiFranciscanScrapers,
    //...clallamHHSScraper,
    // ...multicareScrapers,
    ...multicareDirectScrapers,
    ...timeTapScraper,
    ...peaceHealthScrapers,
    // new MemberPlusFamilyHealthScraper(),
    // new ARCpointLabsScraper(),
  ];

  const scraperHandler = ScraperHandler(...scrapers);
  await scraperHandler(event, context, callback);
};
