#!/bin/bash

## print range of inline arguments
# for val in $@
# do
#     echo $val
# done

# functions
someFunc(){
    echo $0
    echo ${payload}
    echo  "$(dirname "$0")"
    echo running function with $1 and $2
    return 0
}


someFunc javad