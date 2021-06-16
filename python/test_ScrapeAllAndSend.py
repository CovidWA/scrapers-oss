#!/usr/bin/env python
# pytest cannot have an __init__.py in it
# https://stackoverflow.com/questions/41748464/pytest-cannot-import-module-while-python-can
# https://blog.methodsconsultants.com/posts/pytesting-your-python-package/
# to move pytests down we need a setup.py to tell it how to find packages

import sys
import logging

from ScrapeAllAndSend import handler



# for expected exceptions, trap them
# https://stackoverflow.com/questions/33920322/how-do-i-test-exceptions-and-errors-using-pytest
# https://docs.pytest.org/en/stable/assert.html
# for unexpected exception just let it fun
# https://improveandrepeat.com/2020/11/python-friday-46-testing-exceptions-in-pytest/
def test_ScrapeAllAndSend():
    """Test the overall main scraper."""
    handler(event=None, context=None)


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    logging.debug(f"looking for import at {sys.path}")
    test_ScrapeAllAndSend()
