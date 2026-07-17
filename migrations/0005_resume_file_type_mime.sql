SET lock_timeout = '5s';

ALTER TABLE resumes
    ALTER COLUMN file_type TYPE VARCHAR(255);

UPDATE resumes
SET file_type = 'application/pdf'
WHERE file_type IS NOT NULL
  AND lower(btrim(file_type)) IN ('pdf', '.pdf');

UPDATE resumes
SET file_type = 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
WHERE file_type IS NOT NULL
  AND lower(btrim(file_type)) IN ('docx', '.docx');
