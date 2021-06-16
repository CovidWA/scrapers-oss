import {SignUpGeniusScraper} from './signUpGeniusBase';

export class ArlingtonMunicipalAirportScraper extends SignUpGeniusScraper {
  constructor() {
    super('Arlington Municipal Airport', 'ArlingtonMunicipalAirport');
    this.url =
      'https://www.signupgenius.com/tabs/13577df01a0cfedc5ac5-vaccine3';
  }
}
