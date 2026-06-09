import { yfRequest } from './client';

/** The 11 Yahoo sectors, exactly as Yahoo's assetProfile returns them. */
export const CANONICAL_SECTORS: string[] = [
  'Basic Materials',
  'Communication Services',
  'Consumer Cyclical',
  'Consumer Defensive',
  'Energy',
  'Financial Services',
  'Healthcare',
  'Industrials',
  'Real Estate',
  'Technology',
  'Utilities',
];

/**
 * Flattened Yahoo industry display strings (em-dash `—` is intentional —
 * Yahoo returns it literally; matching it lets a fetched value preselect).
 * Source: go-yfinance v1.3.0 SectorIndustryMapping.
 */
export const CANONICAL_INDUSTRIES: string[] = [
  // Basic Materials
  'Specialty Chemicals', 'Gold', 'Building Materials', 'Copper', 'Steel',
  'Agricultural Inputs', 'Chemicals', 'Other Industrial Metals & Mining',
  'Lumber & Wood Production', 'Aluminum', 'Other Precious Metals & Mining',
  'Coking Coal', 'Paper & Paper Products', 'Silver',
  // Communication Services
  'Advertising Agencies', 'Broadcasting', 'Electronic Gaming & Multimedia',
  'Entertainment', 'Internet Content & Information', 'Publishing', 'Telecom Services',
  // Consumer Cyclical
  'Apparel Manufacturing', 'Apparel Retail', 'Auto & Truck Dealerships',
  'Auto Manufacturers', 'Auto Parts', 'Department Stores', 'Footwear & Accessories',
  'Furnishings, Fixtures & Appliances', 'Gambling', 'Home Improvement Retail',
  'Internet Retail', 'Leisure', 'Lodging', 'Luxury Goods',
  'Packaging & Containers', 'Personal Services', 'Recreational Vehicles',
  'Residential Construction', 'Resorts & Casinos', 'Restaurants',
  'Specialty Retail', 'Textile Manufacturing', 'Travel Services',
  // Consumer Defensive
  'Beverages—Brewers', 'Beverages—Non-Alcoholic', 'Beverages—Wineries & Distilleries',
  'Confectioners', 'Discount Stores', 'Education & Training Services',
  'Farm Products', 'Food Distribution', 'Grocery Stores',
  'Household & Personal Products', 'Packaged Foods', 'Tobacco',
  // Energy
  'Oil & Gas Drilling', 'Oil & Gas E&P', 'Oil & Gas Equipment & Services',
  'Oil & Gas Integrated', 'Oil & Gas Midstream', 'Oil & Gas Refining & Marketing',
  'Thermal Coal', 'Uranium',
  // Financial Services
  'Asset Management', 'Banks—Diversified', 'Banks—Regional', 'Capital Markets',
  'Credit Services', 'Financial Conglomerates', 'Financial Data & Stock Exchanges',
  'Insurance Brokers', 'Insurance—Diversified', 'Insurance—Life',
  'Insurance—Property & Casualty', 'Insurance—Reinsurance', 'Insurance—Specialty',
  'Mortgage Finance', 'Shell Companies',
  // Healthcare
  'Biotechnology', 'Diagnostics & Research', 'Drug Manufacturers—General',
  'Drug Manufacturers—Specialty & Generic', 'Health Information Services',
  'Healthcare Plans', 'Medical Care Facilities', 'Medical Devices',
  'Medical Instruments & Supplies', 'Medical Distribution', 'Pharmaceutical Retailers',
  // Industrials
  'Aerospace & Defense', 'Airlines', 'Airports & Air Services',
  'Building Products & Equipment', 'Business Equipment & Supplies', 'Conglomerates',
  'Consulting Services', 'Electrical Equipment & Parts', 'Engineering & Construction',
  'Farm & Heavy Construction Machinery', 'Industrial Distribution',
  'Infrastructure Operations', 'Integrated Freight & Logistics', 'Marine Shipping',
  'Metal Fabrication', 'Pollution & Treatment Controls', 'Railroads',
  'Rental & Leasing Services', 'Security & Protection Services',
  'Specialty Business Services', 'Specialty Industrial Machinery',
  'Staffing & Employment Services', 'Tools & Accessories', 'Trucking', 'Waste Management',
  // Real Estate
  'Real Estate—Development', 'Real Estate Services', 'Real Estate—Diversified',
  'REIT—Healthcare Facilities', 'REIT—Hotel & Motel', 'REIT—Industrial',
  'REIT—Office', 'REIT—Residential', 'REIT—Retail', 'REIT—Mortgage',
  'REIT—Specialty', 'REIT—Diversified',
  // Technology
  'Communication Equipment', 'Computer Hardware', 'Consumer Electronics',
  'Electronic Components', 'Electronics & Computer Distribution',
  'Information Technology Services', 'Scientific & Technical Instruments',
  'Semiconductor Equipment & Materials', 'Semiconductors',
  'Software—Application', 'Software—Infrastructure', 'Solar',
  // Utilities
  'Utilities—Diversified', 'Utilities—Independent Power Producers',
  'Utilities—Regulated Electric', 'Utilities—Regulated Gas',
  'Utilities—Regulated Water', 'Utilities—Renewable',
];

export type ClassificationOption = { label: string; value: string };

/**
 * Build the dropdown option list: canonical values first (in their given
 * order), then any stored/current values not already canonical, de-duped and
 * sorted. Empty/null entries are dropped. Labels equal values.
 */
export function mergeClassificationOptions(
  canonical: string[],
  stored: Array<string | null | undefined>,
  current: string | null | undefined,
): ClassificationOption[] {
  const canonicalSet = new Set(canonical);
  const extras = new Set<string>();
  for (const v of [...stored, current]) {
    if (v && !canonicalSet.has(v)) {
      extras.add(v);
    }
  }
  const ordered = [...canonical, ...Array.from(extras).sort()];
  return ordered.map((v) => ({ label: v, value: v }));
}

/** Manual override of sector and/or industry for a (instrument, portfolio) pair. */
export function setClassification(
  instrumentId: string,
  portfolioId: string,
  patch: { sector?: string | null; industry?: string | null },
) {
  return yfRequest<{ instrument_id: string; portfolio_id: string }>(
    `/classification/${encodeURIComponent(instrumentId)}`,
    { method: 'PATCH', body: { portfolio_id: portfolioId, ...patch } },
  );
}
