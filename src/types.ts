export type GradeDetail = {
  key: string;
  label: string;
  value: string;
  max: number;
  displayScore: string;
  ratio: number;
  pending: boolean;
  tone?: string;
  comment?: string;
  commentAuthor?: string;
};

export type GradeCard = {
  key: string;
  label: string;
  value: string;
  displayValue: string;
  tone?: string;
  comment?: string;
  commentAuthor?: string;
  details?: GradeDetail[];
};

export type GradeTable = {
  key: string;
  label: string;
  sheetName: string;
  kind: string;
  complete: boolean;
  status?: string;
  cards?: GradeCard[];
};

export type StudentStatus = {
  ab1: number;
  ab2: number;
  average: number;
  approved: boolean;
};

export type GradeResult = {
  exam: string;
  matricula: string;
  name: string;
  tables: GradeTable[];
  studentStatus?: StudentStatus;
};

export type SessionUser = {
  matricula: string;
  name: string;
};

export type GradeCache = Partial<Record<'ab1' | 'ab2', GradeResult>>;
