#!/usr/bin/env python
# https://hg.sr.ht/~danmur/pytest-shell/rev/tip
# does not install correctly a bash fixture

# def test_bin_build_sh(bash):
# """Test the build.sh that creates the lambda zip file"""
# bash.run_script_inline(['cd bin'])
# bash.run_functions('build.sh')
# bash.path_exists('../lambda_stage')

from subprocess import call
from os import chdir, path


# https://stackoverflow.com/questions/13745648/running-bash-script-from-within-python
# https://stackoverflow.com/questions/1432924/python-change-the-scripts-working-directory-to-the-scripts-own-directory
def test_lambda_build():
    """Test the build.sh that creates the lambda data."""
    chdir("bin")
    assert call("./build.sh") == 0, "Build script failed"
    assert path.exists("../lambda_stage"), "No generate lambda output"


if __name__ == "__main__":
    test_lambda_build()
