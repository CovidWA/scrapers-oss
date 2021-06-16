# Copyright 2021 CovidWA
from Common import Status, SaveHtmlToTable
from typing import Tuple, List

#Abstract Class defining the blueprint for all scrapers

class ScraperBase:

    # Decorators wrap a function inside another function, see Common.py
    # It allows the scraped HTML (3rd arg) to be saved
    @SaveHtmlToTable
    def MakeGetRequest(self) -> Tuple[List[str], Status, str]:
        # Should return Keys (array), case (enum-string), ScrapedHtml (string)
        # There are multiple keys in case the scraper checks a single status 
        # for multiple locations, therefore multiple keys.
        pass