
from Common import Status, SaveHtmlToTable
from typing import Tuple, List, Set

class DummyScraper():


    def __init__(self, keys, case, html, scraperTags = set()):

        self.Keys = keys
        self.case = case
        self.html = html
        self.scraperTags = scraperTags

    @SaveHtmlToTable
    def MakeGetRequest(self)-> Tuple[List[str], Status, str, Set[str]]:

        return self.Keys, self.case, self.html, self.scraperTags
