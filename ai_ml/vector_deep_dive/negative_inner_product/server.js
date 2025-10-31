const express = require('express');
const cors = require('cors');
const multer = require('multer');
const { Pool } = require('pg');
const OpenAI = require('openai');
const fs = require('fs');
const path = require('path');
const ffmpeg = require('fluent-ffmpeg');
require('dotenv').config();

const app = express();

const uploadsDir = path.join(__dirname, 'uploads');
const storedAudioDir = path.join(__dirname, 'stored_audio');
if (!fs.existsSync(uploadsDir)) {
  fs.mkdirSync(uploadsDir, { recursive: true });
  console.log('Created uploads directory');
}
if (!fs.existsSync(storedAudioDir)) {
  fs.mkdirSync(storedAudioDir, { recursive: true });
  console.log('Created stored_audio directory');
}

const storage = multer.diskStorage({
  destination: function (req, file, cb) {
    cb(null, uploadsDir);
  },
  filename: function (req, file, cb) {
    const uniqueSuffix = Date.now() + '-' + Math.round(Math.random() * 1E9);
    cb(null, 'audio-' + uniqueSuffix + path.extname(file.originalname));
  }
});

const upload = multer({ 
  storage: storage,
  limits: {
    fileSize: 25 * 1024 * 1024
  }
});

const openai = new OpenAI({
  apiKey: process.env.OPENAI_API_KEY,
});

const pool = new Pool({
  connectionString: "postgres://root@localhost:26257/defaultdb?sslmode=disable",
});

app.use(cors());
app.use(express.json());

function convertToMp3(inputPath) {
  return new Promise((resolve, reject) => {
    const outputPath = inputPath.replace(path.extname(inputPath), '.mp3');
        
    ffmpeg(inputPath)
      .toFormat('mp3')
      .audioCodec('libmp3lame')
      .audioBitrate('128k')
      .on('start', (cmd) => console.log('FFmpeg command:', cmd))
      .on('end', () => {
        resolve(outputPath);
      })
      .on('error', (err) => {
        console.error('FFmpeg error:', err);
        reject(err);
      })
      .save(outputPath);
  });
}

async function transcribeAudio(filePath) {
  let convertedPath = null;
  try {
    console.log('Original file size:', fs.statSync(filePath).size, 'bytes');
    
    convertedPath = await convertToMp3(filePath);
    
    console.log('Converted file size:', fs.statSync(convertedPath).size, 'bytes');
    
    const audioFile = fs.createReadStream(convertedPath);
    const transcription = await openai.audio.transcriptions.create({
      file: audioFile,
      model: 'whisper-1',
    });
    
    return transcription.text;
  } finally {
    if (convertedPath && fs.existsSync(convertedPath)) {
      fs.unlinkSync(convertedPath);
    }
  }
}

async function generateEmbedding(text) {
  const response = await openai.embeddings.create({
    model: 'text-embedding-3-small',
    input: text,
  });
  return response.data[0].embedding;
}

app.post('/api/store', upload.single('audio'), async (req, res) => {
  let filePath;
  let storedFilePath;
  try {
    if (!req.file) {
      throw new Error('No audio file uploaded');
    }
    
    filePath = req.file.path;
    
    const transcription = await transcribeAudio(filePath);
    const embedding = await generateEmbedding(transcription);
    
    const storedFilename = `voice-${Date.now()}.mp3`;
    storedFilePath = path.join(storedAudioDir, storedFilename);
    fs.copyFileSync(filePath, storedFilePath);
    
    const result = await pool.query(
      `INSERT INTO voice (transcription, vec, file_name)
       VALUES ($1, $2, $3)
       RETURNING id`,
      [transcription, JSON.stringify(embedding), storedFilename]
    );
    
    res.json({
      success: true,
      id: result.rows[0].id,
      transcription,
      embedding_dimension: embedding.length
    });
  } catch (error) {
    console.error('Error:', error);
    res.status(500).json({ error: error.message, stack: error.stack });
  } finally {
    if (filePath && fs.existsSync(filePath)) {
      fs.unlinkSync(filePath);
    }
  }
});

app.post('/api/search', upload.single('audio'), async (req, res) => {
  let filePath;
  try {
    if (!req.file) {
      throw new Error('No audio file uploaded');
    }
    
    filePath = req.file.path;
    
    const transcription = await transcribeAudio(filePath);
    const embedding = await generateEmbedding(transcription);
    
    const result = await pool.query(
      `WITH similarity_calc AS (
          SELECT 
            id, 
            transcription, 
            file_name,
            created_at,
            (vec <#> $1::vector) * -1 as similarity
          FROM voice
        )
        SELECT *
        FROM similarity_calc
        WHERE similarity >= 0.3
        ORDER BY similarity DESC
        LIMIT 5;`,
      [JSON.stringify(embedding)]
    );
    
    res.json({
      success: true,
      query: transcription,
      results: result.rows,
    });
  } catch (error) {
    console.error('Error:', error);
    res.status(500).json({ error: error.message, stack: error.stack });
  } finally {
    if (filePath && fs.existsSync(filePath)) {
      fs.unlinkSync(filePath);
    }
  }
});

const PORT = process.env.PORT || 3001;
app.listen(PORT, () => {
  console.log(`Server running on port ${PORT}`);
  console.log(`Uploads directory: ${uploadsDir}`);
  console.log(`Stored audio directory: ${storedAudioDir}`);
});