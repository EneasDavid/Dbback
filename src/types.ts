export type Column = {
  key: string;
  label: string;
  value: string;
  comment?: string;
  commentAuthor?: string;
};

export type ActivityItem = {
  key: string;
  subtopic: string;
  notaMaxima: string;
  notaAlcancada: string;
  comentario?: string;
  comentarioAutor?: string;
};

export type GradeResult = {
  exam: string;
  matricula: string;
  name: string;
  tables: GradeTable[];
};

export type SessionUser = {
  matricula: string;
  name: string;
};

export type GradeTable = {
  key: string;
  label: string;
  sheetName: string;
  kind: string;
  complete: boolean;
  status?: string; // "Encerrado" ou "Não encerrado"
  columns: Column[];
  items?: ActivityItem[];
};

export type StudentStatus = {
  ab1: number;
  ab2: number;
  average: number;
  approved: boolean;
};

export type GradeCache = Partial<Record<'ab1' | 'ab2', GradeResult>>;
