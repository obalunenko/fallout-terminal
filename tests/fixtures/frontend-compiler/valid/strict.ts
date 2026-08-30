interface StrictFixtureRecord {
  readonly label: string;
  readonly optionalValue?: number;
}

const record: StrictFixtureRecord = { label: 'strict' };

export const strictFixtureLabel: string = record.label;
