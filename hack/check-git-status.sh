#! /usr/bin/env bash

git update-index -q --ignore-submodules --refresh

git diff-files --quiet --ignore-submodules
unstaged=$?

if [ "$unstaged" -ne "0" ]; then
  echo "You have unstaged local changes."
fi

git diff-index --cached --quiet --ignore-submodules HEAD --

staged=$?
if [ "$staged" -ne "0" ]; then
  echo "You have uncommitted changes."
fi

exit 0


