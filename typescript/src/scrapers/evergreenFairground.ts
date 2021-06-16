import {SignUpGeniusScraper} from './signUpGeniusBase';

export class EvergreenFairgroundScraper extends SignUpGeniusScraper {
  constructor() {
    super('Evergreen Fairground', 'evergreen_state');
    this.url =
      'https://www.signupgenius.com/tabs/13577df01a0cfedc5ac5-vaccine2';
  }
}
