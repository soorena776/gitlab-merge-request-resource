#!/bin/bash
read -r -d '' payload << EOM
{
  "source": {
    "uri": "git://some-uri",
    "branch": "develop",
    "private_key": "..."
  },
  "version": { "ref": "61cebf" }
}
EOM

uri="$(jq -r '.source.uri // ""' < "${payload}")"

