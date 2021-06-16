class PrepModResult():
    def __init__(self, uniqueKey, case, name, address, content_url, county='', city='', avail_count=0, scraperTags=set()):
        self.key = uniqueKey
        self.case = case
        self.name = name
        self.address = address
        self.content_url = content_url
        self.county = county
        self.city = city
        self.avail_count = avail_count      # Availability count is cumulative
        self.scraperTags = scraperTags      # Use a set so there will be no dups

    def __repr__(self):
        return f"{self.case}, {self.name}, {self.county}, {self.city}\n"

    def __str__(self):
        return f"{self.case}, {self.name}, {self.county}, {self.city}\n"
