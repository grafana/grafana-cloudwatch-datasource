import { debounce } from 'lodash';
import { useEffect, useRef, useState } from 'react';

import { Icon, Input } from '@grafana/ui';

type Props = {
  ariaLabel: string;
  placeholder: string;
  searchPhrase: string;
  searchFn: (searchPhrase: string) => void;
};

export const SelectionSearchInput = ({ ariaLabel, placeholder, searchFn, searchPhrase }: Props) => {
  const [searchFilter, setSearchFilter] = useState(searchPhrase);
  const searchFnRef = useRef(searchFn);
  const debouncedSearchRef = useRef<ReturnType<typeof debounce> | null>(null);

  useEffect(() => {
    searchFnRef.current = searchFn;
  }, [searchFn]);

  useEffect(() => {
    const debouncedSearch = debounce((nextSearchPhrase: string) => searchFnRef.current(nextSearchPhrase), 600);
    debouncedSearchRef.current = debouncedSearch;

    return () => {
      debouncedSearch?.cancel();
      debouncedSearchRef.current = null;
    };
  }, []);

  return (
    <Input
      aria-label={ariaLabel}
      prefix={<Icon name="search" />}
      value={searchFilter}
      onChange={(event) => {
        const nextSearchPhrase = event.currentTarget.value;
        setSearchFilter(nextSearchPhrase);
        debouncedSearchRef.current?.(nextSearchPhrase);
      }}
      placeholder={placeholder}
    />
  );
};
