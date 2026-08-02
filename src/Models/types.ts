export type GradeDetail = {
  key: string;
  label: string;
  value: string;
  max: number;
  displayScore: string;
  ratio: number;
  pending: boolean;
  percentage?: boolean;
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
  scoreless?: boolean;
  status?: string;
  schemaStatus?: string;
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
  active?: boolean;
  schemaStatus?: string;
  tables: GradeTable[];
  studentStatus?: StudentStatus;
  // Preenchido quando essa prova especifica falhou ao buscar dados (ex.
  // limite de requisicoes do Google), sem derrubar as demais provas de uma
  // resposta com varias provas (/api/grades/all).
  error?: string;
};

export type SessionUser = {
  matricula: string;
  name: string;
  schemaStatus?: string;
};

export type GradeCache = Record<string, GradeResult | undefined>;
export type GradeResults = Record<string, GradeResult>;
