<script>
 const poems = [
  {
    "title": "Untitled",
    "text": "Too many people live\nWith too little poetry.\nAfter all, if our lives\nAre stories, what makes you think\nThat one not worth telling\nCould be one worth living?",
    "id": "poem-1"
  },
  {
    "title": "The Verge of a Hill",
    "text": "I don't see any berries,\nbut I can smell them\nup ahead, over that next hill.\n\nAnd the hill beyond that?\nI keep my mind from running\ntoo far beyond my body.\n\nFor flavor on the way down\nwill never be as sweet\nas the scent on the way up.\n\nCoffee, incense, and wood\nare the only scents\nwaiting for me below.",
    "id": "poem-2"
  },
  {
    "title": "Minus Something",
    "text": "Try to balance the equation,\nAnd it seems to transform.\nThe completeness slips away.\nIt's minus something.\n\nThe one loose thread\nThat splits and twists\nInto a whole new sweater.\nIt's minus something.\n\nA riddle whose answer\nIs a question whose answer\nIs a riddle once again.\nIt's minus something.\n\nThe poem that knows itself,\nIn the empty theater \nOf the reader's mind.\nIt's minus something.",
    "id": "poem-3"
  },
  {
    "title": "Oldening",
    "text": "Where has my poetry gone?\nIt used to echo through\nThe empty halls of my mind\nFor days on end.\nNow it's all full with\nCinder blocks of information,\nDoors I've closed,\nAnd wisdom like damp mold.",
    "id": "poem-4"
  },
  {
    "title": "Worriers Refrain",
    "text": "Victory, I do not ride for thee.\nToo many hopeful dead\nAre gathered at your feet.\n\nLoss, I do not fear thee,\nAny more than I fear the shadow\nStretching out from my boot heels.\n\nI hope only for the fate of a tree,\nWhose roots find loam,\nWhose leaves find sun.",
    "id": "poem-5"
  },
  {
    "title": "Dick & Sally",
    "text": "Seventy six million years have gone\nAnd seventy six million more will come.\nYou only had seventy six years together.\n\nIt's less than we hoped, but more than we deserve.\n\nIt's enough time to feel joy.\nAnd enough time to feel sad.\nAnd enough time to grow tired.\n\nIt's less than we hoped, but more than we deserve.\n\nIn your wake are the people you knew,\nWith all their bitterness and petty feuds,\nTheir battles and little wins.\n\nIt's less than we hoped, but more than we deserve.",
    "id": "poem-6"
  },
  {
    "title": "The Inverted Man",
    "text": "The man who lived upside down,\nNever knew which way to climb.\nTo solid ground above him?\nOr to the open sky below?\n\nGravity never took to him well,\nSaid he wasn't much to hold onto.\nOr maybe a distant star, too distant to see,\nHad fixed its pull on this inverted man.\n\nBecause even with the Queens mattress \nBelow him, he couldn't seem to fall into bed.\nWhen November comes again, the stone in his shoe\nAnd the trick up his sleeve won't be enough.\n\nSo grab a fistful of carpet and hold on,\nOr tie yourself to the porch. \nWhen the house shakes it will probably\nShake him loose.",
    "id": "poem-7"
  },
  {
    "title": "A Strange Day",
    "text": "When the eye of God fixates on you,\nFor a moment. And then looks away.\nWhen you're just a mortal man.\nYour world becomes small\nAnd you see the tip of it rushing towards you.\nLike an arrow you noticed too late\nTo avoid, or bargain with.\nThe shadow of his gaze stretches out\nFar in front of you, until\nEvery step you take is in shadow.\n\t\t",
    "id": "poem-8"
  },
  {
    "title": "Love Path",
    "text": "I welcome bad luck,\nbecause good news makes me nervous.\nAnd I got the worst of it today, that\nLove is like being buried alive.\nOr drowning in different versions\nof yourself.\nOr seeing yourself in a movie\nyou didn't write.\nOr walking on a trail\nWith no beginning or end.",
    "id": "poem-9"
  },
  {
    "title": "Never Knew",
    "text": "In the parking lots of our lives, \nUnder the overpass, \nThe asphalt is illuminated\nWith the ideas of those we \nNever knew. \n\nIn the gridlock of our minds, \nCaught by air traffic, \nOur thoughts are coursed\nAlong the paths of those we\nNever knew. \n\nIn the videos of our childhood, \nRecorded in streaked static, \nOur fears are projected\nOnto the younger selves we\nNever knew. \n\t\t",
    "id": "poem-10"
  },
  {
    "title": "Day After A Good Day",
    "text": "I left flower tomes for you in the sky,\nradiant abysma.\nThe universe no longer speaks to me\nin feathered ellipses.\nAnd the forest no longer welcomes me\nto its dark cornered caves. \nAnd silver songs no longer gather\nand drip from above.\nThe goddess is wet with sunlight \nand stretching herself out into the morning. \nEmpty dawn, hypnotic lofty ephemera. ",
    "id": "poem-11"
  },
  {
    "title": "Thoughts on a Goodlife",
    "text": "i want to speak in\nraw shimmering dream tones\nsee the future that is\ndark and immense\nlet the darklight drip\ninto my minds eye\nbathe in the pouring water\nfrom the open palms of\na dying goddess in heat\nsail through the emberhot \nnetwork ideawire\ntear down into threads\nthe twine that i was\nsewn up into before i\nwas a child\ntaste the slippery \neffervescent thoughtstuff\nand die in the silk\nsheet web of bliss\t",
    "id": "poem-12"
  },
  {
    "title": "Poem for Chiara",
    "text": "You\nare morning,\nand like the brightening edge of the world\ndo illuminate\nwhat was once dusted in darkness.\n\nYou\nare an ocean,\nand as the warm wind carries a bird\ndo hold me\nlike a wild beast and a baby.\n\nYou\nare the tide,\nand as the moon circles the earth,\ndo pull me in",
    "id": "poem-13"
  },
  {
    "title": "One Day",
    "text": "One day\nWe will all\nbe destroyed\nAnd integrated back\nInto the sky--\nThe endless equation\nfrom which we derived.",
    "id": "poem-14"
  },
  {
    "title": "Dear Gwen",
    "text": "Today was a dark day\nand to it I say:\n\tGood Riddance.\nNo more waiting for tomorrow\nor its brass beams of light.\nDear Gwen, thanks for the thoughts\nbut I'll be alright.\n\nWe wandered through \nthe empty halls of our\nneighborhood at night,\nand you told me that time \nis a fire that left us\nsifting through ash.\n\t\t...did you mean that?\n\nThese lonely days I think \nof you to say:\n\tGoodbye.\nSomeday in the light maybe \nour minds will meet.\nDear Gwen, are you still out there\nin the rivers and streets?\n\nWe sat on the watertower \nand watched as the wind \ncut through the trees below,\nand you said it would \ncarry us above the damp\nsoil and into the sky.\n\t...that dream was truly a lie.\n\nI’ll be waiting for a lighter day\nto hear you say:\n\tWelcome.\nTomorrow is coming and no-\nbody will be spared.\nDear Gwen, you go ahead\nand I will meet you there.\t",
    "id": "poem-15"
  },
  {
    "title": "Mallory",
    "text": "The Mallard is a bird\nyou can only find once.\nIn the rubble around you it\nflies and if you\nflay it or stay with it\nit’s all that you get.\nNotice the colors\nand cream and the back\nof its arc.\nYou are now living\nin the age of the lark.",
    "id": "poem-16"
  },
  {
    "title": "Caught My Reflecton",
    "text": "im a clear clever private pervert\nin the night and im known\nas something other than what i am\nthat is more me than ill ever be.\ni have a crazy dream where\nim more than i was before\nand my crooked teeth would\nsit straight in my crooked jaw.\ni climbed up a wave that\nwhittled away at the world\nas it slipped slowly by\nso i could see the passing of time.\nAfter assessing all ills \nI knew that despite all that is \nbroken, everythings fine.",
    "id": "poem-17"
  }
];
</script>

<svelte:head><title>Poetry — Casey Clyde</title><meta name="description" content="Poems by Casey Clyde." /></svelte:head>

<div class="poetry-page" id="top">
 <header class="masthead"><a href="/">Home</a><h1>Poetry</h1><a href="/about">About</a></header>
 <main id="collection">
  <div class="poems">
   {#each poems as poem}
    <article id={poem.id} aria-label={poem.title === 'Untitled' ? poem.text.split('\n')[0] : poem.title}>
     {#if poem.title !== 'Untitled'}<h2>{poem.title}</h2>{/if}
     <p class="verse">{poem.text}</p>
    </article>
   {/each}
  </div>
 </main>
</div>
<style>
 .poetry-page{background:#f5f1e7;color:#253d34;min-height:100vh}
 .masthead{height:66px;max-width:820px;margin:auto;display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid #d1cbbb}
 .masthead a{font:12px var(--font-mono);color:inherit;text-decoration:none}
 h1{font:normal 26px Georgia,serif;color:#253d34;margin:0;text-align:left}
 main{max-width:640px;margin:0 auto;padding:70px 0 24px}
 article{padding:0 0 54px;margin:0 0 54px;border-bottom:1px solid #d1cbbb}
 article:first-child .verse{font-size:clamp(22px,2.4vw,29px);line-height:1.65}
 article:last-child{border:0;margin-bottom:0}
 h2{font:normal clamp(29px,3vw,36px)/1.2 Georgia,'Times New Roman',serif;letter-spacing:-.5px;color:#253d34;margin:0 0 28px;text-align:left}
 .verse{white-space:pre-wrap;font:19px/1.8 Georgia,'Times New Roman',serif;color:#323b31;margin:0;overflow-wrap:break-word;text-align:left}
 a:focus-visible{outline:2px solid #a85439;outline-offset:5px}
 @media(max-width:900px){.masthead{margin:0 28px}main{margin:0 auto;padding:48px 28px 24px}}
 @media(max-width:600px){.verse{font-size:17px}article{margin-bottom:40px;padding-bottom:40px}h2{font-size:30px}}
 @media(max-width:380px){main{padding-left:20px;padding-right:20px}.verse{font-size:16px}}
</style>