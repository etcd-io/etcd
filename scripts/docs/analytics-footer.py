#!/usr/bin/env python
"""
Filename: ensure-analytics.py
Description: Ensures the correct Google Analytics tracking pixel is at the
  footer of each document. This document can be found at scripts/analytics.txt
  It looks like this:

::

    <!-- BEGIN ANALYTICS --> [![Analytics](http://ga-beacon.prod.coreos.systems/ANALYTICS_ID/SITE/ORG/PROJECT/PATH?pixel)]() <!-- END ANALYTICS -->

With ANALYTICS_ID and PROJECT/PATH set to the correct values for a given document.
    
"""

import os
from sys import argv

# Please keep this to one line.
analytics_str = "<!-- BEGIN ANALYTICS --> [![Analytics](http://ga-beacon.prod.coreos.systems/ANALYTICS_ID/SITE/ORG/PROJECT/PATH?pixel)]() <!-- END ANALYTICS -->\n"

# All of this data will be written to the footer
analytics_id = 'UA-42684979-9'
org = 'coreos'
project = 'etcd'
site = 'github.com'

# Complete relative paths only! (./path/to/ignore/dir)
IGNORE = ['./vendor', './.github', './.git']

HELP_TEXT = """\
Usage:
    analytics_footer.py [add|remove]\
"""

def main():
    # Recursively iterate over all files in this directory
    for root, dirs, fnames in os.walk('.'):
        # Generate a list [True, False, ...] where
        #     True -> The root of this file includes an ignore directory string
        #     False -> The root of this file does not include the ignore directory string
        # If any entries in that list are 'True' set to True, else set to False.
        a = [root.startswith(x) for x in IGNORE]
        ignorable = True if True in a else False
        if not ignorable:
            for fname in fnames:
                # Only operate on markdown files
                if fname.endswith('.md'):
                    doc_footer = footer(root, fname)
                    try:
                        if argv[1] == 'add':
                            doc_footer.add()
                            op = 'added'
                        elif argv[1] == 'remove':
                            doc_footer.remove()
                            op = 'deleted'
                        else:
                            print(HELP_TEXT)
                            return
                    except IndexError:
                        print(HELP_TEXT)
                        return
    if op == 'added':
        print('Documents analytics footers updated.')
    elif op == 'deleted':
        print('Document analytics footers removed.')

class footer:
    def __init__(self, root, fname):
        self.fname = fname
        self.root  = root
        self.generate()

    def generate(self):
        """
        Generate the footer string mentioned at the top of the source script.
        """
        # Set current doc to a variable for convenience
        self.curr_doc = os.path.join(self.root, self.fname)

        # This ``if root is not '.' else fname`` business is to avoid an edgecase which results in 
        # urls like '[...]//README.md' and '[...]//CONTRIBUTING.md' in the root of the repo.
        if self.root is not '.':
            filepath = '/'.join(self.root.split('/')[1:]) + '/' + self.fname
        else:
            filepath = self.fname

        self.footer = analytics_str.replace('ANALYTICS_ID', analytics_id) \
                                   .replace('SITE', site) \
                                   .replace('ORG', org) \
                                   .replace('PROJECT', project) \
                                   .replace('PATH', filepath)
        return self.footer

    def remove(self):
        """
        Remove self.footer from self.curr_doc
        """
        with open(self.curr_doc, 'r+') as f:
            f_arr = f.readlines()
            f_str = ''.join(f_arr)
            if f_arr[-1].endswith('<!-- END ANALYTICS -->\n'):
                del f_arr[-1] # Remove last line (footer)
                del f_arr[-1] # Remove last empty newline
            contents = ''.join(f_arr)
        self._write(contents)

    def add(self):
        """
        Open the file. We will either:
        - Add the footer
        - Update the footer
        """
        with open(self.curr_doc, 'r+') as f:
            f_arr = f.readlines()
            f_str = ''.join(f_arr)
            # File has bad footer
            if f_arr[-1] != self.footer:
                # Incorrect footer
                if f_arr[-1].endswith('<!-- END ANALYTICS -->\n'):
                    f_arr[-1] = self.footer
                    contents = ''.join(f_arr)
                # No footer
                elif f_arr[-1] != self.footer:
                    contents = ''.join(f_arr) + '\n' + self.footer
            else:
                contents = f_str
        self._write(contents)

    def _write(self, contents):
        """
        Writes `contents` to self.curr_doc.
        """
        with open(self.curr_doc, 'w') as f:
            f.write(contents)

if __name__ == '__main__':
    main()
