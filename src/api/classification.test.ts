import {
  CANONICAL_SECTORS,
  CANONICAL_INDUSTRIES,
  mergeClassificationOptions,
} from './classification';

describe('CANONICAL_SECTORS', () => {
  it('has the 11 Yahoo sectors', () => {
    expect(CANONICAL_SECTORS).toHaveLength(11);
    expect(CANONICAL_SECTORS).toContain('Technology');
    expect(CANONICAL_SECTORS).toContain('Financial Services');
  });
});

describe('CANONICAL_INDUSTRIES', () => {
  it('is the flattened, de-duped, non-empty industry list', () => {
    expect(CANONICAL_INDUSTRIES.length).toBeGreaterThan(140);
    expect(new Set(CANONICAL_INDUSTRIES).size).toBe(CANONICAL_INDUSTRIES.length);
    expect(CANONICAL_INDUSTRIES).toContain('Software—Infrastructure');
  });
});

describe('mergeClassificationOptions', () => {
  it('keeps canonical order, then appends stored/current extras sorted', () => {
    const opts = mergeClassificationOptions(
      ['Technology', 'Energy'],
      ['My Custom', 'Energy', null, undefined, ''],
      'Another Custom',
    );
    expect(opts.map((o) => o.value)).toEqual([
      'Technology',
      'Energy',
      'Another Custom',
      'My Custom',
    ]);
    expect(opts.every((o) => o.label === o.value)).toBe(true);
  });

  it('does not duplicate current when it is canonical', () => {
    const opts = mergeClassificationOptions(['Technology', 'Energy'], [], 'Technology');
    expect(opts.map((o) => o.value)).toEqual(['Technology', 'Energy']);
  });

  it('handles a null current and empty stored', () => {
    const opts = mergeClassificationOptions(['Technology'], [], null);
    expect(opts.map((o) => o.value)).toEqual(['Technology']);
  });
});
