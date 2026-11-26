import json
import sys
from io import BytesIO

from docling.datamodel.base_models import DocumentStream, InputFormat
from docling.datamodel.pipeline_options import PdfPipelineOptions, TableFormerMode
from docling.document_converter import DocumentConverter, PdfFormatOption

options = PdfPipelineOptions(do_ocr=False, do_table_structure=True, enable_remote_services=False)
options.table_structure_options.mode = TableFormerMode.ACCURATE
converter = DocumentConverter(format_options={InputFormat.PDF: PdfFormatOption(pipeline_options=options)})
source = DocumentStream(name="confirmation.pdf", stream=BytesIO(sys.stdin.buffer.read()))
result = converter.convert(source, max_num_pages=20, max_file_size=8 << 20, raises_on_error=False)
if result.status.value != "success":
    sys.exit(1)
document = result.document
json.dump({
    "tables": [table.data.model_dump() for table in document.tables],
    "texts": [item.text for item in document.texts],
}, sys.stdout)
